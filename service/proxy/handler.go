package proxy

import (
	"0E7/service/config"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var manager *Manager

func initManager() {
	if manager == nil {
		// default expiration is not used directly; we pass per-request TTL.
		manager = NewManager(0, 1*time.Minute, config.Proxy_retain_duration)
	}
}

func RegisterRoutes(r *gin.Engine) {
	initManager()
	r.Any("/proxy/:ttl/*upstream", handleProxy)
}

func ListCacheEntries() []CacheEntryMeta {
	initManager()
	return manager.List()
}

func handleProxy(c *gin.Context) {

	// parse TTL duration
	ttlStr := c.Param("ttl")
	if ttlStr == "" {
		c.String(http.StatusBadRequest, "missing ttl")
		return
	}
	var ttl time.Duration
	var err error
	if ttlStr == "0s" || ttlStr == "0" {
		ttl = 0
	} else {
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil || ttl < 0 {
			c.String(http.StatusBadRequest, "invalid ttl")
			return
		}
	}

	upstreamRaw := strings.TrimPrefix(c.Param("upstream"), "/")
	if upstreamRaw == "" {
		c.String(http.StatusBadRequest, "missing upstream url")
		return
	}
	// ensure it's a full URL
	if !strings.HasPrefix(upstreamRaw, "http://") && !strings.HasPrefix(upstreamRaw, "https://") {
		upstreamRaw = "http://" + upstreamRaw
	}
	upstreamURL, err := url.Parse(upstreamRaw)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid upstream url")
		return
	}

	// read body for cache key and forwarding
	bodyBytes, _ := ioutil.ReadAll(c.Request.Body)
	c.Request.Body = ioutil.NopCloser(bytes.NewReader(bodyBytes))

	method := c.Request.Method

	// Try cache
	if ttl > 0 {
		if resp, _, ok := manager.Get(method, upstreamURL.String(), bodyBytes); ok && resp != nil {
			// If expired but within stale window, refresh in background
			if time.Now().After(resp.ExpiresAt) {
				go func(method string, u *url.URL, body []byte, ttl time.Duration) {
					_ = refreshOnce(method, u, body, ttl)
				}(method, upstreamURL, bodyBytes, ttl)
			}
			// increment hits when actually serving from cache
			manager.Increment(method, upstreamURL.String(), bodyBytes)
			// add cache meta headers
			h := cloneHeader(resp.Header)
			remain := int(time.Until(resp.ExpiresAt).Seconds())
			if remain < 0 {
				remain = 0
			}
			h.Set("proxy-cache-ttl", strconv.Itoa(remain))
			h.Set("proxy-cache-expire", resp.ExpiresAt.UTC().Format(time.RFC3339))
			writeResponse(c, resp.StatusCode, h, bytes.NewReader(resp.Body))
			return
		}
	}

	// Forward request
	client := &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second}
	forwardReq, err := http.NewRequestWithContext(c.Request.Context(), method, upstreamURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		c.String(http.StatusBadGateway, "forward request build failed")
		return
	}
	copyHeaders(c.Request.Header, &forwardReq.Header)
	forwardReq.Host = upstreamURL.Host

	resp, err := client.Do(forwardReq)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// cache only if enabled and status allowed
	shouldCache := ttl > 0 && (!config.Proxy_cache_2xx_only || (resp.StatusCode >= 200 && resp.StatusCode < 300))
	hdr := cloneHeader(resp.Header)
	if shouldCache {
		now := time.Now()
		cr := &CachedResponse{
			StatusCode: resp.StatusCode,
			Header:     cloneHeader(resp.Header),
			Body:       body,
			CachedAt:   now,
			TTL:        ttl,
			ExpiresAt:  now.Add(ttl),
		}
		manager.Set(method, upstreamURL.String(), bodyBytes, cr)
		remain := int(time.Until(cr.ExpiresAt).Seconds())
		if remain < 0 {
			remain = 0
		}
		hdr.Set("proxy-cache-ttl", strconv.Itoa(remain))
		hdr.Set("proxy-cache-expire", cr.ExpiresAt.UTC().Format(time.RFC3339))
	} else {
		// 即使未缓存（如 ttl=0 或状态不满足策略），也返回提示头
		remain := int(ttl.Seconds())
		if remain < 0 {
			remain = 0
		}
		hdr.Set("proxy-cache-ttl", strconv.Itoa(remain))
		hdr.Set("proxy-cache-expire", time.Now().Add(ttl).UTC().Format(time.RFC3339))
	}
	writeResponse(c, resp.StatusCode, hdr, bytes.NewReader(body))
}

func refreshOnce(method string, u *url.URL, body []byte, ttl time.Duration) error {
	client := &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second}
	req, err := http.NewRequest(method, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	shouldCache := ttl > 0 && (!config.Proxy_cache_2xx_only || (resp.StatusCode >= 200 && resp.StatusCode < 300))
	if shouldCache {
		cr := &CachedResponse{
			StatusCode: resp.StatusCode,
			Header:     cloneHeader(resp.Header),
			Body:       b,
			CachedAt:   time.Now(),
			TTL:        ttl,
			ExpiresAt:  time.Now().Add(ttl),
		}
		manager.Set(method, u.String(), body, cr)
	}
	return nil
}

func writeResponse(c *gin.Context, status int, hdr http.Header, body io.Reader) {
	for k, vals := range hdr {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(status)
	if body != nil {
		io.Copy(c.Writer, body)
	}
}

func copyHeaders(src http.Header, dst *http.Header) {
	for k, v := range src {
		// skip hop-by-hop headers
		lk := strings.ToLower(k)
		if lk == "connection" || lk == "proxy-connection" || lk == "keep-alive" || lk == "proxy-authenticate" || lk == "proxy-authorization" || lk == "te" || lk == "trailers" || lk == "transfer-encoding" || lk == "upgrade" {
			continue
		}
		for _, vv := range v {
			dst.Add(k, vv)
		}
	}
}

func cloneHeader(h http.Header) http.Header {
	cl := http.Header{}
	for k, v := range h {
		vv := make([]string, len(v))
		copy(vv, v)
		cl[k] = vv
	}
	return cl
}
