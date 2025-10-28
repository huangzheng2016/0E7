package pcap

import (
	"bufio"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/andybalholm/brotli"
)

const DecompressionSizeLimit = int64(streamdoc_limit)

// AddFingerprints 已废弃，不再使用
func AddFingerprints(cookies []*http.Cookie, fingerPrints map[uint32]bool) {
	// 功能已禁用
}

// Parse and simplify every item in the flow. Items that were not successfuly
// parsed are left as-is.
//
// If we manage to simplify a flow, the new data is placed in flowEntry.data
func ParseHttpFlow(flow *FlowEntry) {
	for idx := 0; idx < len(flow.Flow); idx++ {
		flowItem := &flow.Flow[idx]
		// 从B64解码数据
		data, err := base64.StdEncoding.DecodeString(flowItem.B64)
		if err != nil {
			continue
		}
		reader := bufio.NewReader(strings.NewReader(string(data)))

		if flowItem.From == "c" {
			// HTTP Request
			_, err := http.ReadRequest(reader)
			if err != nil {
				continue
			}

		} else if flowItem.From == "s" {
			// Parse HTTP Response
			res, err := http.ReadResponse(reader, nil)
			if err != nil {
				continue
			}

			// Substitute body
			encoding := res.Header["Content-Encoding"]
			if encoding == nil || len(encoding) == 0 {
				// If we don't find an encoding header, it is either not valid,
				// or already in plain text. In any case, we don't have to edit anything.
				continue
			}

			var newReader io.Reader
			if err != nil {
				// Failed to fully read the body. Bail out here
				continue
			}
			switch encoding[0] {
			case "gzip":
				newReader, err = handleGzip(res.Body)
				break
			case "br":
				newReader, err = handleBrotili(res.Body)
				break
			case "deflate":
				//UNTODO; verify this is correct
				newReader, err = handleGzip(res.Body)
				break
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
					flowItem.B64 = base64.StdEncoding.EncodeToString(replacement)
					flow.Size = new_size
				}
			}
		}
	}

	// Fingerprints 功能已禁用
}

func handleGzip(r io.Reader) (io.Reader, error) {
	return gzip.NewReader(r)
}

func handleBrotili(r io.Reader) (io.Reader, error) {
	reader := brotli.NewReader(r)
	return reader, nil
}
