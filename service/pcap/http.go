package pcap

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const DecompressionSizeLimit = int64(streamdoc_limit)

func ParseHttpFlow(flow *FlowEntry) {
	ensureTag := func(tag string) {
		for _, t := range flow.Tags {
			if t == tag {
				return
			}
		}
		flow.Tags = append(flow.Tags, tag)
	}

	wsActive := false
	// WebSocket 有状态解析（按方向）
	type wsParser struct {
		tailBuf           []byte
		messageBuf        []byte
		haveMessage       bool
		curRSV1           bool
		perMessageDeflate bool
	}
	parseWS := func(p *wsParser, chunk []byte) []byte {
		var out bytes.Buffer
		p.tailBuf = append(p.tailBuf, chunk...)
		b := p.tailBuf
		i := 0
		for {
			if i+2 > len(b) {
				break
			}
			b1 := b[i]
			b2 := b[i+1]
			fin := (b1 & 0x80) != 0
			rsv1 := (b1 & 0x40) != 0
			opcode := b1 & 0x0F
			masked := (b2 & 0x80) != 0
			payloadLen := uint64(b2 & 0x7F)
			i += 2
			if payloadLen == 126 {
				if i+2 > len(b) {
					i -= 2
					break
				}
				payloadLen = uint64(binary.BigEndian.Uint16(b[i : i+2]))
				i += 2
			} else if payloadLen == 127 {
				if i+8 > len(b) {
					i -= 2
					break
				}
				payloadLen = binary.BigEndian.Uint64(b[i : i+8])
				i += 8
			}
			var maskKey []byte
			if masked {
				if i+4 > len(b) {
					i -= 2
					break
				}
				maskKey = b[i : i+4]
				i += 4
			}
			if i+int(payloadLen) > len(b) {
				i -= 2
				break
			}
			payload := make([]byte, int(payloadLen))
			copy(payload, b[i:i+int(payloadLen)])
			i += int(payloadLen)
			if masked && len(maskKey) == 4 {
				for j := 0; j < len(payload); j++ {
					payload[j] ^= maskKey[j%4]
				}
			}
			// 聚合分片
			if opcode != 0x0 { // 新消息开始（非 CONTINUATION）
				p.messageBuf = p.messageBuf[:0]
				p.haveMessage = true
				p.curRSV1 = rsv1
			}
			if p.haveMessage {
				p.messageBuf = append(p.messageBuf, payload...)
			}
			if fin && p.haveMessage {
				// 完整消息
				final := p.messageBuf
				// permessage-deflate 解压（若启用且 RSV1）
				if p.perMessageDeflate && p.curRSV1 {
					if dec, ok := tryDecompressPMD(final); ok {
						final = dec
					}
				}
				// 优先尝试 HTTP/1 解析，失败则回退文本
				if req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(final))); err == nil {
					if dump, derr := httputil.DumpRequest(req, true); derr == nil {
						out.Write(dump)
						out.WriteString("\r\n\r\n")
					} else {
						out.WriteString(bytesToUTF8(final))
						out.WriteString("\r\n\r\n")
					}
				} else if resp, err2 := http.ReadResponse(bufio.NewReader(bytes.NewReader(final)), nil); err2 == nil {
					if dump, derr := httputil.DumpResponse(resp, true); derr == nil {
						out.Write(dump)
						out.WriteString("\r\n\r\n")
					} else {
						out.WriteString(bytesToUTF8(final))
						out.WriteString("\r\n\r\n")
					}
				} else {
					out.WriteString(bytesToUTF8(final))
					out.WriteString("\r\n\r\n")
				}
				p.haveMessage = false
				p.curRSV1 = false
				p.messageBuf = p.messageBuf[:0]
			}
		}
		// 保留未消费的尾部
		p.tailBuf = b[i:]
		return out.Bytes()
	}
	wsClient := &wsParser{}
	wsServer := &wsParser{}
	var wsFrames int
	var wsBytes int
	h2Active := false
	var grpcMsgs int
	var grpcBytes int

	for idx := 0; idx < len(flow.Flow); idx++ {
		flowItem := &flow.Flow[idx]
		// 从B64解码数据
		data, err := base64.StdEncoding.DecodeString(flowItem.B64)
		if err != nil {
			continue
		}
		reader := bufio.NewReader(bytes.NewReader(data))

		// h2c 识别（明文 HTTP/2 前言）
		if !h2Active && bytes.HasPrefix(data, []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")) {
			h2Active = true
			ensureTag("HTTP2")
			// 将 h2c 前言后的帧尽力解析并重写为 HTTP/2.0 文本
			if rebuilt, ok := rebuildH2cAsHTTPText(data); ok {
				flowItem.B64 = base64.StdEncoding.EncodeToString(rebuilt)
				// 更新 size 防止超限
				delta := len(rebuilt) - len(data)
				if flow.Size+delta <= streamdoc_limit {
					flow.Size += delta
				}
			}
		}

		// gRPC 识别（字符串快速探测）
		if bytes.Contains(bytes.ToLower(data), []byte("application/grpc")) {
			ensureTag("GRPC")
		}

		if h2Active {
			// 尝试把包含 HEADERS/DATA 的明文 h2 帧转成人类可读文本
			if rebuilt, ok := rebuildH2FramesChunkAsText(data); ok {
				flowItem.B64 = base64.StdEncoding.EncodeToString(rebuilt)
			}
		} else if flowItem.From == "c" {
			// HTTP Request
			_, err := http.ReadRequest(reader)
			if err != nil {
				continue
			}
			// WebSocket 升级识别与握手请求摘要
			br := bufio.NewReader(bytes.NewReader(data))
			if req, e := http.ReadRequest(br); e == nil {
				if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") &&
					strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade") {
					wsActive = true
					ensureTag("WEBSOCKET")
					// 输出握手请求摘要（与响应风格保持一致）
					summary := bytes.NewBuffer(nil)
					fmt.Fprintf(summary, "WebSocket handshake request: %s %s\r\n", req.Method, req.URL.RequestURI())
					if v := req.Host; v != "" {
						fmt.Fprintf(summary, "Host: %s\r\n", v)
					}
					if v := req.Header.Get("Sec-WebSocket-Key"); v != "" {
						fmt.Fprintf(summary, "Sec-WebSocket-Key: %s\r\n", v)
					}
					if v := req.Header.Get("Sec-WebSocket-Version"); v != "" {
						fmt.Fprintf(summary, "Sec-WebSocket-Version: %s\r\n", v)
					}
					if v := req.Header.Get("Origin"); v != "" {
						fmt.Fprintf(summary, "Origin: %s\r\n", v)
					}
					if v := req.Header.Get("Sec-WebSocket-Protocol"); v != "" {
						fmt.Fprintf(summary, "Sec-WebSocket-Protocol: %s\r\n", v)
					}
					if v := req.Header.Get("Sec-WebSocket-Extensions"); v != "" {
						fmt.Fprintf(summary, "Sec-WebSocket-Extensions: %s\r\n", v)
						if strings.Contains(strings.ToLower(v), "permessage-deflate") {
							wsClient.perMessageDeflate = true
							wsServer.perMessageDeflate = true
						}
					}
					flowItem.B64 = base64.StdEncoding.EncodeToString(summary.Bytes())
				}
			}
		} else if flowItem.From == "s" {
			// Parse HTTP Response
			res, err := http.ReadResponse(reader, nil)
			if err != nil {
				continue
			}

			// WebSocket 握手响应识别（优先）
			if res.StatusCode == 101 && strings.EqualFold(res.Header.Get("Upgrade"), "websocket") &&
				strings.Contains(strings.ToLower(res.Header.Get("Connection")), "upgrade") {
				summary := bytes.NewBuffer(nil)
				fmt.Fprintf(summary, "WebSocket handshake response: 101 Switching Protocols\r\n")
				if v := res.Header.Get("Sec-WebSocket-Accept"); v != "" {
					fmt.Fprintf(summary, "Sec-WebSocket-Accept: %s\r\n", v)
				}
				if v := res.Header.Get("Sec-WebSocket-Protocol"); v != "" {
					fmt.Fprintf(summary, "Sec-WebSocket-Protocol: %s\r\n", v)
				}
				if v := res.Header.Get("Sec-WebSocket-Extensions"); v != "" {
					fmt.Fprintf(summary, "Sec-WebSocket-Extensions: %s\r\n", v)
					if strings.Contains(strings.ToLower(v), "permessage-deflate") {
						wsClient.perMessageDeflate = true
						wsServer.perMessageDeflate = true
					}
				}
				flowItem.B64 = base64.StdEncoding.EncodeToString(summary.Bytes())
				wsActive = true
				ensureTag("WEBSOCKET")
				continue
			}

			// Substitute body
			encoding := res.Header["Content-Encoding"]
			if len(encoding) == 0 {
				// If we don't find an encoding header, it is either not valid,
				// or already in plain text. In any case, we don't have to edit anything.
				continue
			}

			var newReader io.Reader
			switch encoding[0] {
			case "gzip":
				newReader, err = handleGzip(res.Body)
			case "br":
				newReader, err = handleBrotili(res.Body)
			case "deflate":
				newReader, err = handleDeflate(res.Body)
			default:
				// Skipped, unknown or identity encoding
				continue
			}

			// Replace the reader to allow for in-place decompression
			if err == nil && newReader != nil {
				// Limit the reader to prevent potential decompression bombs
				res.Body = io.NopCloser(io.LimitReader(newReader, DecompressionSizeLimit))
				// invalidate the content length, since decompressing the body will change its value.
				res.ContentLength = -1
				replacement, err := httputil.DumpResponse(res, true)
				if err != nil {
					// HTTPUtil failed us, continue without replacing anything.
					continue
				}
				// This can exceed the mongo document limit, so we need to make sure
				// the replacement will fit
				new_size := flow.Size + (len(replacement) - len(data))
				if new_size <= streamdoc_limit {
					// 追加一个空行作为跨报文分隔
					withSep := append(replacement, []byte("\r\n\r\n")...)
					flowItem.B64 = base64.StdEncoding.EncodeToString(withSep)
					flow.Size = new_size
				}
			}
		}

		// WebSocket 帧统计（升级后对后续双向流量做轻量统计）
		if wsActive {
			// 有状态解析，重组消息并按需解压
			var summary []byte
			if flowItem.From == "c" {
				summary = parseWS(wsClient, data)
			} else {
				summary = parseWS(wsServer, data)
			}
			if len(summary) > 0 {
				flowItem.B64 = base64.StdEncoding.EncodeToString(summary)
			}
			fcnt, fbytes := countWebSocketFrames(data)
			wsFrames += fcnt
			wsBytes += fbytes
		}

		// gRPC 消息统计（基于5字节前缀的 framing）
		if bytes.Contains(bytes.ToLower(data), []byte("application/grpc")) || h2Active {
			mcnt, mbytes := countGrpcMessages(data)
			if mcnt > 0 {
				// 用摘要文本替换该段数据，便于阅读
				summary := []byte(fmt.Sprintf("gRPC chunk: messages=%d, bytes=%d\r\n", mcnt, mbytes))
				flowItem.B64 = base64.StdEncoding.EncodeToString(summary)
			}
			grpcMsgs += mcnt
			grpcBytes += mbytes
		}
	}

}

func handleGzip(r io.Reader) (io.Reader, error) {
	return gzip.NewReader(r)
}

func handleBrotili(r io.Reader) (io.Reader, error) {
	reader := brotli.NewReader(r)
	return reader, nil
}

func handleDeflate(r io.Reader) (io.Reader, error) {
	if zr, err := zlib.NewReader(r); err == nil {
		return zr, nil
	}
	rc := flate.NewReader(r)
	return rc, nil
}

// 轻量 WebSocket 帧统计：尽力而为（不处理掩码和分片细节）
func countWebSocketFrames(b []byte) (int, int) {
	// RFC6455 minimal parse for stats
	i := 0
	frames := 0
	total := 0
	for i+2 <= len(b) {
		if len(b)-i < 2 {
			break
		}
		b1 := b[i]
		b2 := b[i+1]
		i += 2
		masked := (b2 & 0x80) != 0
		var payloadLen uint64 = uint64(b2 & 0x7F)
		if payloadLen == 126 {
			if i+2 > len(b) {
				break
			}
			payloadLen = uint64(binary.BigEndian.Uint16(b[i : i+2]))
			i += 2
		} else if payloadLen == 127 {
			if i+8 > len(b) {
				break
			}
			payloadLen = binary.BigEndian.Uint64(b[i : i+8])
			i += 8
		}
		if masked {
			if i+4 > len(b) {
				break
			}
			i += 4 // skip masking key
		}
		// skip payload
		if i+int(payloadLen) > len(b) {
			break
		}
		i += int(payloadLen)
		_ = b1
		frames++
		total += int(payloadLen)
	}
	return frames, total
}

// gRPC 消息统计：按照5字节（1字节压缩标志+4字节大端长度）提取
func countGrpcMessages(b []byte) (int, int) {
	i := 0
	msgs := 0
	total := 0
	for i+5 <= len(b) {
		// 允许在任意位置尝试同步
		compressed := b[i]
		_ = compressed
		n := int(binary.BigEndian.Uint32(b[i+1 : i+5]))
		if n < 0 || i+5+n > len(b) {
			i++
			continue
		}
		msgs++
		total += n
		i += 5 + n
	}
	return msgs, total
}

// 将任意字节转换为 UTF-8 文本（非法字节替换为 �）
func bytesToUTF8(b []byte) string {
	var out []rune
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			out = append(out, '�')
			b = b[1:]
			continue
		}
		out = append(out, r)
		b = b[size:]
	}
	return string(out)
}

// 判断是否文本类内容
func isTextLike(ct string) bool {
	if ct == "" {
		return false
	}
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	if strings.Contains(ct, "json") || strings.Contains(ct, "xml") || strings.Contains(ct, "yaml") {
		return true
	}
	return false
}

// 将 h2c 前言后的帧尽力解析成 HTTP/2.0 文本
func rebuildH2cAsHTTPText(b []byte) ([]byte, bool) {
	preface := []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
	if !bytes.HasPrefix(b, preface) {
		return nil, false
	}
	return rebuildH2FramesChunkAsText(b[len(preface):])
}

// 解析一段 HTTP/2 明文字节（HEADERS/DATA），输出近似 HTTP/1 样式文本（首行 HTTP/2.0）
func rebuildH2FramesChunkAsText(b []byte) ([]byte, bool) {
	fr := http2.NewFramer(bytes.NewBuffer(nil), bytes.NewReader(b))
	// 解码 HEADERS 使用 HPACK
	dec := hpack.NewDecoder(4096, nil)
	var out bytes.Buffer
	wrote := false
	// 记录每个 StreamID 的 Content-Encoding，便于对后续 DATA 解压
	encByStream := make(map[uint32]string)
	ctByStream := make(map[uint32]string)
	pathByStream := make(map[uint32]string)
	schemeByStream := make(map[uint32]string)
	authByStream := make(map[uint32]string)
	for i := 0; i < 32; i++ { // 限制帧数，防止阻塞
		f, err := fr.ReadFrame()
		if err != nil {
			break
		}
		switch hf := f.(type) {
		case *http2.HeadersFrame:
			fields, err := dec.DecodeFull(hf.HeaderBlockFragment())
			if err != nil {
				continue
			}
			// 构建 HTTP/2.0 首行与头部
			pseudo := map[string]string{}
			headers := http.Header{}
			for _, kv := range fields {
				k := kv.Name
				v := kv.Value
				if len(k) > 0 && k[0] == ':' {
					pseudo[k] = v
				} else {
					headers.Add(k, v)
				}
			}
			// 记录本 stream 的 content-encoding
			if ce := headers.Get("content-encoding"); ce != "" {
				encByStream[hf.StreamID] = strings.ToLower(ce)
			} else {
				delete(encByStream, hf.StreamID)
			}
			// 记录 Content-Type 及 URL 组成要素
			if ct := headers.Get("content-type"); ct != "" {
				ctByStream[hf.StreamID] = strings.ToLower(ct)
			} else {
				delete(ctByStream, hf.StreamID)
			}
			if p := pseudo[":path"]; p != "" {
				pathByStream[hf.StreamID] = p
			}
			if sc := pseudo[":scheme"]; sc != "" {
				schemeByStream[hf.StreamID] = sc
			}
			if au := pseudo[":authority"]; au != "" {
				authByStream[hf.StreamID] = au
			}
			if m, ok := pseudo[":method"]; ok {
				// Request 形式
				path := pseudo[":path"]
				if path == "" {
					path = "/"
				}
				full := path
				if sc, okSc := schemeByStream[hf.StreamID]; okSc {
					if au, okAu := authByStream[hf.StreamID]; okAu {
						full = fmt.Sprintf("%s://%s%s", sc, au, path)
					}
				}
				fmt.Fprintf(&out, "%s %s HTTP/2.0\r\n", m, full)
			} else if st, ok := pseudo[":status"]; ok {
				// Response 形式
				fmt.Fprintf(&out, "HTTP/2.0 %s\r\n", st)
			} else {
				// 未识别
				continue
			}
			// 输出常规头
			for k, vals := range headers {
				for _, v := range vals {
					fmt.Fprintf(&out, "%s: %s\r\n", k, v)
				}
			}
			out.WriteString("\r\n")
			wrote = true
		case *http2.DataFrame:
			// 如果有 DATA，尝试按记录的 encoding 解压并输出前缀
			data := hf.Data()
			enc := encByStream[hf.StreamID]
			// gRPC 优先摘要
			if ct := ctByStream[hf.StreamID]; strings.Contains(ct, "application/grpc") {
				mcnt, mbytes := countGrpcMessages(data)
				method := pathByStream[hf.StreamID]
				if method == "" {
					method = "/"
				}
				fmt.Fprintf(&out, "gRPC stream: method=%s, messages=%d, bytes=%d\r\n", method, mcnt, mbytes)
			} else {
				var plain []byte
				if enc != "" {
					if decompressed, ok := tryDecompressByEncoding(enc, data); ok {
						plain = decompressed
					} else {
						fmt.Fprintf(&out, "[HTTP/2 DATA len=%d encoding=%s undecoded]\r\n", len(data), enc)
						wrote = true
						break
					}
				} else {
					plain = data
				}
				ct := ctByStream[hf.StreamID]
				if isTextLike(ct) || strings.Contains(ct, "json") {
					out.Write(plain)
				} else {
					fmt.Fprintf(&out, "[HTTP/2 DATA binary len=%d]\r\n", len(plain))
				}
			}
			wrote = true
		default:
			// 忽略其他帧，仅做识别
		}
	}
	if !wrote {
		return nil, false
	}
	return out.Bytes(), true
}

// 根据 Content-Encoding 尝试解压一段数据
func tryDecompressByEncoding(encoding string, data []byte) ([]byte, bool) {
	var r io.Reader
	var err error
	switch encoding {
	case "gzip":
		r, err = handleGzip(bytes.NewReader(data))
	case "br":
		r, err = handleBrotili(bytes.NewReader(data))
	case "deflate":
		r, err = handleDeflate(bytes.NewReader(data))
	default:
		return nil, false
	}
	if err != nil || r == nil {
		return nil, false
	}
	// 加总限流，防止炸弹
	limited := io.LimitReader(r, DecompressionSizeLimit)
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, false
	}
	return out, true
}

// WebSocket permessage-deflate：对单条消息的原始 DEFLATE 负载解压（带上限）
func tryDecompressPMD(data []byte) ([]byte, bool) {
	r := flate.NewReader(bytes.NewReader(data))
	if r == nil {
		return nil, false
	}
	defer r.Close()
	limited := io.LimitReader(r, DecompressionSizeLimit)
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, false
	}
	return out, true
}

// WebSocket 帧可读摘要（行式）
func summarizeWebSocketFrames(b []byte, dir string) []byte {
	var out bytes.Buffer
	i := 0
	count := 0
	for i+2 <= len(b) && count < 32 { // 限制每块最多 32 帧
		b1 := b[i]
		b2 := b[i+1]
		i += 2
		fin := (b1 & 0x80) != 0
		opcode := b1 & 0x0F
		masked := (b2 & 0x80) != 0
		rsv1 := (b1 & 0x40) != 0 // 压缩标记（permessage-deflate）
		var payloadLen uint64 = uint64(b2 & 0x7F)
		if payloadLen == 126 {
			if i+2 > len(b) {
				break
			}
			payloadLen = uint64(binary.BigEndian.Uint16(b[i : i+2]))
			i += 2
		} else if payloadLen == 127 {
			if i+8 > len(b) {
				break
			}
			payloadLen = binary.BigEndian.Uint64(b[i : i+8])
			i += 8
		}
		var maskKey []byte
		if masked {
			if i+4 > len(b) {
				break
			}
			maskKey = b[i : i+4]
			i += 4
		}
		if i+int(payloadLen) > len(b) {
			break
		}
		payload := b[i : i+int(payloadLen)]
		i += int(payloadLen)
		if masked && len(maskKey) == 4 {
			for j := 0; j < len(payload); j++ {
				payload[j] ^= maskKey[j%4]
			}
		}
		var name string
		switch opcode {
		case 0x0:
			name = "CONTINUATION"
		case 0x1:
			name = "TEXT"
		case 0x2:
			name = "BINARY"
		case 0x8:
			name = "CLOSE"
		case 0x9:
			name = "PING"
		case 0xA:
			name = "PONG"
		default:
			name = fmt.Sprintf("OP%02x", opcode)
		}
		finStr := ""
		if fin {
			finStr = ", FIN"
		}
		text := bytesToUTF8(payload)
		compStr := ""
		if rsv1 {
			compStr = ", RSV1"
		}
		fmt.Fprintf(&out, "WebSocket frame: %s, dir=%s, len=%d%s%s, text=\"%s\"\r\n", name, dir, payloadLen, finStr, compStr, text)
		count++
	}
	return out.Bytes()
}
