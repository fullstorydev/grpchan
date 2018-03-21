package internal

import (
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryServerTransportStream implements grpc.ServerTransportStream and can be
// used by unary calls to collect headers and trailers from a handler.
type UnaryServerTransportStream struct {
	// Name is the full method name in "/service/method" format.
	Name string

	mu       sync.Mutex
	hdrs     metadata.MD
	hdrsSent bool
	tlrs     metadata.MD
	tlrsSent bool
}

func (sts *UnaryServerTransportStream) Method() string {
	return sts.Name
}

func (sts *UnaryServerTransportStream) Finish() {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	sts.hdrsSent = true
	sts.tlrsSent = true
}

func (sts *UnaryServerTransportStream) SetHeader(md metadata.MD) error {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	return sts.setHeaderLocked(md)
}

func (sts *UnaryServerTransportStream) SendHeader(md metadata.MD) error {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	if err := sts.setHeaderLocked(md); err != nil {
		return err
	}
	sts.hdrsSent = true
	return nil
}

func (sts *UnaryServerTransportStream) setHeaderLocked(md metadata.MD) error {
	if sts.hdrsSent {
		return fmt.Errorf("headers already sent")
	}
	if sts.hdrs == nil {
		sts.hdrs = metadata.MD{}
	}
	for k, v := range md {
		sts.hdrs[k] = append(sts.hdrs[k], v...)
	}
	return nil
}

func (sts *UnaryServerTransportStream) GetHeaders() metadata.MD {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	return sts.hdrs
}

func (sts *UnaryServerTransportStream) SetTrailer(md metadata.MD) error {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	if sts.tlrsSent {
		return fmt.Errorf("trailers already sent")
	}
	if sts.tlrs == nil {
		sts.tlrs = metadata.MD{}
	}
	for k, v := range md {
		sts.tlrs[k] = append(sts.tlrs[k], v...)
	}
	return nil
}

func (sts *UnaryServerTransportStream) GetTrailers() metadata.MD {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	return sts.tlrs
}

// ServerTransportStream implements grpc.ServerTransportStream and wraps a
// grpc.ServerStream, delegating most calls to it.
type ServerTransportStream struct {
	// Name is the full method name in "/service/method" format.
	Name string
	// Stream is the underlying stream to which header and trailer calls are
	// delegated.
	Stream grpc.ServerStream
}

func (sts *ServerTransportStream) Method() string {
	return sts.Name
}

func (sts *ServerTransportStream) SetHeader(md metadata.MD) error {
	return sts.Stream.SetHeader(md)
}

func (sts *ServerTransportStream) SendHeader(md metadata.MD) error {
	return sts.Stream.SendHeader(md)
}

// If the underlying stream provides a TrySetTrailer(metadata.MD) error method,
// it will be used to set trailers. Otherwise, the normal SetTrailer(metadata.MD)
// method will be used and a nil error will always be returned.
func (sts *ServerTransportStream) SetTrailer(md metadata.MD) error {
	type trailerWithErrors interface {
		TrySetTrailer(md metadata.MD) error
	}
	if t, ok := sts.Stream.(trailerWithErrors); ok {
		return t.TrySetTrailer(md)
	}
	sts.Stream.SetTrailer(md)
	return nil
}
