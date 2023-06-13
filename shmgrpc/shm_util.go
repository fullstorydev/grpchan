package shmgrpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
)

type QueueInfo struct {
	QueuePath        string
	QueueId          uint
	QueueReqType     uint
	QueueReqTypeMeta uint
	QueueRespType    uint
}

type ShmMessage struct {
	Method   string            `json:"method"`
	Context  context.Context   `json:"context"`
	Headers  map[string][]byte `json:"headers,omitempty"`
	Trailers map[string][]byte `json:"trailers,omitempty"`
	// Payload interface{}     `protobuf:"bytes,3,opt,name=method,proto3" json:"payload"`
}

// headersFromContext returns HTTP request headers to send to the remote host
// based on the specified context. GRPC clients store outgoing metadata into the
// context, which is translated into headers. Also, a context deadline will be
// propagated to the server via GRPC timeout metadata.
func headersFromContext(ctx context.Context) http.Header {
	h := http.Header{}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		toHeaders(md, h, "")
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		millis := int64(timeout / time.Millisecond)
		if millis <= 0 {
			millis = 1
		}
		h.Set("GRPC-Timeout", fmt.Sprintf("%dm", millis))
	}
	return h
}

var reservedHeaders = map[string]struct{}{
	"accept-encoding":   {},
	"connection":        {},
	"content-type":      {},
	"content-length":    {},
	"keep-alive":        {},
	"te":                {},
	"trailer":           {},
	"transfer-encoding": {},
	"upgrade":           {},
}

func toHeaders(md metadata.MD, h http.Header, prefix string) {
	// binary headers must be base-64-encoded
	for k, vs := range md {
		lowerK := strings.ToLower(k)
		if _, ok := reservedHeaders[lowerK]; ok {
			// ignore reserved header keys
			continue
		}
		isBin := strings.HasSuffix(lowerK, "-bin")
		for _, v := range vs {
			if isBin {
				v = base64.URLEncoding.EncodeToString([]byte(v))
			}
			h.Add(prefix+k, v)
		}
	}
}
