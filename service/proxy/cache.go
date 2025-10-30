package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	cache "github.com/patrickmn/go-cache"
)

type CachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	CachedAt   time.Time
	TTL        time.Duration
	ExpiresAt  time.Time
	StaleUntil time.Time
}

type EntryStatus string

const (
	StatusActive  EntryStatus = "active"
	StatusStale   EntryStatus = "stale"
	StatusExpired EntryStatus = "expired"
)

type CacheEntryMeta struct {
	Key            string
	Method         string
	URL            string
	BodyHash       string
	Status         EntryStatus
	CachedAt       time.Time
	ExpiresAt      time.Time
	StaleUntil     time.Time
	TTL            time.Duration
	StatusCode     int
	Hits           int64 // 导出以便 JSON 返回
	BodySnapshot   []byte
	HeaderSnapshot http.Header
}

type Manager struct {
	data   *cache.Cache
	mu     sync.RWMutex
	metas  map[string]*CacheEntryMeta
	retain time.Duration
	stopCh chan struct{}
}

func NewManager(defaultExpiration, cleanupInterval, retain time.Duration) *Manager {
	m := &Manager{
		data:   cache.New(defaultExpiration, cleanupInterval),
		metas:  make(map[string]*CacheEntryMeta),
		retain: retain,
		stopCh: make(chan struct{}),
	}
	go m.metaJanitor(5 * time.Minute)
	return m
}

func (m *Manager) makeKey(method, url string, body []byte) (string, string) {
	// key: METHOD + URL + sha256(body)
	h := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(h[:])
	return method + " " + url + " " + bodyHash, bodyHash
}

func (m *Manager) Get(method, url string, body []byte) (*CachedResponse, *CacheEntryMeta, bool) {
	key, _ := m.makeKey(method, url, body)
	if v, found := m.data.Get(key); found {
		resp := v.(*CachedResponse)
		m.mu.Lock()
		if meta, ok := m.metas[key]; ok {
			meta.Status = StatusActive
			meta.StatusCode = resp.StatusCode
		}
		m.mu.Unlock()
		return resp, m.snapshotMeta(key), true
	}
	// miss: check if meta exists and within stale period
	m.mu.RLock()
	meta, ok := m.metas[key]
	m.mu.RUnlock()
	if ok && time.Now().Before(meta.StaleUntil) {
		if v, found := m.data.Get(key); found {
			return v.(*CachedResponse), m.snapshotMeta(key), true
		}
		// serve from meta snapshot (stale)
		return &CachedResponse{
			StatusCode: meta.StatusCode,
			Header:     cloneHeader(meta.HeaderSnapshot),
			Body:       append([]byte(nil), meta.BodySnapshot...),
			CachedAt:   meta.CachedAt,
			TTL:        meta.TTL,
			ExpiresAt:  meta.ExpiresAt,
		}, m.snapshotMeta(key), true
	}
	return nil, nil, false
}

func (m *Manager) Set(method, url string, body []byte, resp *CachedResponse) {
	key, bodyHash := m.makeKey(method, url, body)
	m.data.Set(key, resp, resp.TTL)
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.metas[key]; ok {
		// 刷新：更新动态字段，保留首次时间与初始 TTL
		existing.Status = StatusActive
		existing.CachedAt = resp.CachedAt
		existing.ExpiresAt = resp.ExpiresAt
		existing.StaleUntil = resp.ExpiresAt.Add(m.retain)
		existing.TTL = resp.TTL
		existing.StatusCode = resp.StatusCode
		existing.BodySnapshot = append(existing.BodySnapshot[:0], resp.Body...)
		existing.HeaderSnapshot = cloneHeader(resp.Header)
	} else {
		meta := &CacheEntryMeta{
			Key:            key,
			Method:         method,
			URL:            url,
			BodyHash:       bodyHash,
			Status:         StatusActive,
			CachedAt:       resp.CachedAt,
			ExpiresAt:      resp.ExpiresAt,
			StaleUntil:     resp.ExpiresAt.Add(m.retain),
			TTL:            resp.TTL,
			StatusCode:     resp.StatusCode,
			BodySnapshot:   append([]byte(nil), resp.Body...),
			HeaderSnapshot: cloneHeader(resp.Header),
		}
		m.metas[key] = meta
	}
}

func (m *Manager) MarkStale(key string, until time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if meta, ok := m.metas[key]; ok {
		meta.Status = StatusStale
		meta.StaleUntil = until
	}
}

func (m *Manager) SetExpired(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if meta, ok := m.metas[key]; ok {
		meta.Status = StatusExpired
	}
}

func (m *Manager) snapshotMeta(key string) *CacheEntryMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if meta, ok := m.metas[key]; ok {
		copy := *meta
		return &copy
	}
	return nil
}

func (m *Manager) List() []CacheEntryMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]CacheEntryMeta, 0, len(m.metas))
	for _, v := range m.metas {
		copy := *v
		copy.Hits = atomic.LoadInt64(&v.Hits)
		list = append(list, copy)
	}
	return list
}

// Close 停止后台清理（预留）
func (m *Manager) Close() {
	close(m.stopCh)
}

// 根据策略清理：now > ExpiresAt + 2*TTL 则删除元信息与缓存值
func (m *Manager) metaJanitor(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			m.mu.Lock()
			for k, meta := range m.metas {
				// 清理阈值：now > ExpiresAt + TTL
				ttl := meta.TTL
				if ttl < 0 {
					ttl = 0
				}
				cutoff := meta.ExpiresAt.Add(ttl)
				if now.After(cutoff) {
					// 从 go-cache 与 metas 中删除
					m.data.Delete(k)
					delete(m.metas, k)
				}
			}
			m.mu.Unlock()
		case <-m.stopCh:
			return
		}
	}
}

// Increment increases hit counter for the given request identity and returns the new value.
func (m *Manager) Increment(method, url string, body []byte) int64 {
	key, _ := m.makeKey(method, url, body)
	m.mu.RLock()
	meta, ok := m.metas[key]
	m.mu.RUnlock()
	if !ok {
		return 0
	}
	return atomic.AddInt64(&meta.Hits, 1)
}

// GetHits returns current hit counter snapshot.
func (m *Manager) GetHits(method, url string, body []byte) int64 {
	key, _ := m.makeKey(method, url, body)
	m.mu.RLock()
	meta, ok := m.metas[key]
	m.mu.RUnlock()
	if !ok {
		return 0
	}
	return atomic.LoadInt64(&meta.Hits)
}
