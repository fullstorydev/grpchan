package grpchan

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Channel is an abstraction of a GRPC transport. With corresponding generated
// code, it can provide an alternate transport to the standard HTTP/2-based one.
// For example, a Channel implementation could instead provide an HTTP 1.1-based
// transport, or an in-process transport.
type Channel interface {
	// InvokeUnary executes a unary RPC, sending the given req message and populating
	// the given resp with the server's reply.
	Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error

	// InvokeStream executes a streaming RPC.
	NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error)
}

// Channel interface matches the relevant methods on ClientConn
var _ Channel = (*grpc.ClientConn)(nil)
