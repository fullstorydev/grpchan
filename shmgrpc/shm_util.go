package shmgrpc

import (
	"context"

	"google.golang.org/grpc/metadata"
)

type QueueInfo struct {
	QueuePath         string
	QueueId           uint
	Qid               uint
	QueueReqType      uint
	QueueReqTypeMeta  uint
	QueueRespType     uint
	QueueRespTypeMeta uint
}

type ShmMessage struct {
	Method  string          `json:"method"`
	Context context.Context `json:"context"`
	// Headers  map[string][]byte `json:"headers,omitempty"`
	// Trailers map[string][]byte `json:"trailers,omitempty"`
	Headers  metadata.MD `json:"headers"`
	Trailers metadata.MD `json:"trailers"`
	Payload  string      `json:"payload"`
	// Payload interface{}     `protobuf:"bytes,3,opt,name=method,proto3" json:"payload"`
}

// headersFromContext returns HTTP request headers to send to the remote host
// based on the specified context. GRPC clients store outgoing metadata into the
// context, which is translated into headers. Also, a context deadline will be
// propagated to the server via GRPC timeout metadata.
func headersFromContext(ctx context.Context) metadata.MD {
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		//great
	}

	return md
}
