package grpchan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fullstorydev/grpchan/internal"
)

func TestInterceptorChainServer_Unary(t *testing.T) {
	for _, testCase := range interceptorChainCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			handlers := HandlerMap{}
			intercepted := testCase.setupServer(handlers)
			th := testChainHandler{t: t}
			intercepted.RegisterService(th.serviceDesc(), 0)
			var req, expectReply string
			var expectHeaders, expectTrailers metadata.MD
			if testCase.unaryIntercepted {
				req = "req"
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("header", "value"))
				expectHeaders = metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
				expectTrailers = metadata.Pairs("trailer", "value", "trailer-A", "value-A", "trailer-B", "value-B", "trailer-C", "value-C")
				expectReply = "reply,C,B,A"
			} else {
				// Need to add stuff to the request and headers, etc that would otherwise be added by
				// interceptors for the test channel to be happy.
				req = "req,A,B,C"
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C"))
				// And we don't expect anything to be added to the response stuff.
				expectHeaders = metadata.Pairs("header", "value")
				expectTrailers = metadata.Pairs("trailer", "value")
				expectReply = "reply"
			}
			var reply string
			dec := func(reqPtr any) error {
				str, ok := reqPtr.(*string)
				if !ok {
					t.Errorf("invalid request type: %T", reqPtr)
				}
				*str = req
				return nil
			}
			var stream testChainStreamServer
			sts := &internal.ServerTransportStream{Name: "/foo/bar", Stream: &stream}
			sd, srv := handlers.QueryService("foo")
			resp, err := sd.Methods[0].Handler(srv, grpc.NewContextWithServerTransportStream(ctx, sts), dec, nil)
			if err != nil {
				t.Fatalf("unexpected RPC error: %v", err)
			}
			reply, ok := resp.(string)
			if !ok {
				t.Errorf("invalid reply type: %T", resp)
			} else if reply != expectReply {
				t.Errorf("unexpected reply: %s", reply)
			}
			if !reflect.DeepEqual(expectHeaders, stream.headers) {
				t.Errorf("unexpected headers: %s", stream.headers)
			}
			if !reflect.DeepEqual(expectTrailers, stream.trailers) {
				t.Errorf("unexpected trailers: %s", stream.trailers)
			}
		})
	}
}

func TestInterceptorChainServer_Stream(t *testing.T) {
	for _, testCase := range interceptorChainCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			handlers := HandlerMap{}
			intercepted := testCase.setupServer(handlers)
			th := testChainHandler{t: t}
			intercepted.RegisterService(th.serviceDesc(), 0)
			var reqSuffix, expectReplySuffix string
			var expectHeaders, expectTrailers metadata.MD
			if testCase.streamIntercepted {
				reqSuffix = ""
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("header", "value"))
				expectHeaders = metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
				expectTrailers = metadata.Pairs("trailer", "value", "trailer-A", "value-A", "trailer-B", "value-B", "trailer-C", "value-C")
				expectReplySuffix = ",C,B,A"
			} else {
				// Need to add stuff to the request and headers, etc that would otherwise be added by
				// interceptors for the test channel to be happy.
				reqSuffix = ",A,B,C"
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C"))
				// And we don't expect anything to be added to the response stuff.
				expectHeaders = metadata.Pairs("header", "value")
				expectTrailers = metadata.Pairs("trailer", "value")
				expectReplySuffix = ""
			}
			sd, srv := handlers.QueryService("foo")
			stream := &testChainStreamServer{
				t:         t,
				ctx:       ctx,
				reqSuffix: reqSuffix,
			}
			err := sd.Streams[0].Handler(srv, stream)
			if err != nil {
				t.Fatalf("unexpected RPC error: %v", err)
			}
			expectedSent := []string{
				"reply3" + expectReplySuffix,
				"reply2" + expectReplySuffix,
				"reply1" + expectReplySuffix,
			}
			if !reflect.DeepEqual(expectedSent, stream.sent) {
				t.Errorf("unexpected sent: %s", stream.sent)
			}
			if !reflect.DeepEqual(expectHeaders, stream.headers) {
				t.Errorf("unexpected headers: %s", stream.headers)
			}
			if !reflect.DeepEqual(expectTrailers, stream.trailers) {
				t.Errorf("unexpected trailers: %s", stream.trailers)
			}
		})
	}
}

func setupServerChainBatch(reg grpc.ServiceRegistrar) grpc.ServiceRegistrar {
	int1 := chainServerInterceptor{id: "A"}
	int2 := chainServerInterceptor{id: "B"}
	int3 := chainServerInterceptor{id: "C"}
	return WithUnaryInterceptors(
		WithStreamInterceptors(
			reg,
			int1.doStream, int2.doStream, int3.doStream,
		),
		int1.doUnary, int2.doUnary, int3.doUnary,
	)
}

func setupServerChainSingles(reg grpc.ServiceRegistrar) grpc.ServiceRegistrar {
	int1 := chainServerInterceptor{id: "A"}
	int2 := chainServerInterceptor{id: "B"}
	int3 := chainServerInterceptor{id: "C"}
	return WithStreamInterceptors(
		WithUnaryInterceptors(
			WithStreamInterceptors(
				WithUnaryInterceptors(
					WithStreamInterceptors(
						WithUnaryInterceptors(
							reg,
							int3.doUnary,
						),
						int3.doStream,
					),
					int2.doUnary,
				),
				int2.doStream,
			),
			int1.doUnary,
		),
		int1.doStream,
	)
}

func setupServerChainPairs(reg grpc.ServiceRegistrar) grpc.ServiceRegistrar {
	int1 := chainServerInterceptor{id: "A"}
	int2 := chainServerInterceptor{id: "B"}
	int3 := chainServerInterceptor{id: "C"}
	return WithInterceptor(
		WithInterceptor(
			WithInterceptor(
				reg,
				int3.doUnary, int3.doStream,
			),
			int2.doUnary, int2.doStream,
		),
		int1.doUnary, int1.doStream,
	)
}

func setupServerChainUnaryOnly(reg grpc.ServiceRegistrar) grpc.ServiceRegistrar {
	int1 := chainServerInterceptor{id: "A"}
	int2 := chainServerInterceptor{id: "B"}
	int3 := chainServerInterceptor{id: "C"}
	return WithUnaryInterceptors(
		WithUnaryInterceptors(
			WithUnaryInterceptors(
				reg,
				int3.doUnary,
			),
			int2.doUnary,
		),
		int1.doUnary,
	)
}

func setupServerChainStreamOnly(reg grpc.ServiceRegistrar) grpc.ServiceRegistrar {
	int1 := chainServerInterceptor{id: "A"}
	int2 := chainServerInterceptor{id: "B"}
	int3 := chainServerInterceptor{id: "C"}
	return WithStreamInterceptors(
		WithStreamInterceptors(
			WithStreamInterceptors(
				reg,
				int3.doStream,
			),
			int2.doStream,
		),
		int1.doStream,
	)
}

type chainServerInterceptor struct {
	id string
}

func (c *chainServerInterceptor) doUnary(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		md.Append("header-"+c.id, "value-"+c.id)
		ctx = metadata.NewIncomingContext(ctx, md)
	}
	str, ok := req.(string)
	if !ok {
		return nil, status.Errorf(codes.Internal, "unexpected request type: %T", req)
	}
	str += "," + c.id
	if err := grpc.SetHeader(ctx, metadata.Pairs("header-"+c.id, "value-"+c.id)); err != nil {
		return nil, err
	}
	if grpc.SetTrailer(ctx, metadata.Pairs("trailer-"+c.id, "value-"+c.id)) != nil {
		return nil, err
	}
	reply, err := handler(ctx, str)
	if err != nil {
		return nil, err
	}
	str, ok = reply.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", reply)
	}
	str += "," + c.id
	return str, nil
}

func (c *chainServerInterceptor) doStream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	md, ok := metadata.FromIncomingContext(ss.Context())
	if ok {
		md.Append("header-"+c.id, "value-"+c.id)
		ctx = metadata.NewIncomingContext(ctx, md)
	}
	ss = &chainServerStream{ServerStream: ss, ctx: ctx, id: c.id}
	if err := ss.SetHeader(metadata.Pairs("header-"+c.id, "value-"+c.id)); err != nil {
		return err
	}
	if err := handler(srv, ss); err != nil {
		return err
	}
	ss.SetTrailer(metadata.Pairs("trailer-"+c.id, "value-"+c.id))
	return nil
}

type chainServerStream struct {
	grpc.ServerStream
	ctx context.Context
	id  string
}

func (c *chainServerStream) Context() context.Context {
	return c.ctx
}

func (c *chainServerStream) SendMsg(msg any) error {
	str, ok := msg.(string)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected message type: %T", msg)
	}
	str += "," + c.id
	return c.ServerStream.SendMsg(str)
}

func (c *chainServerStream) RecvMsg(msg any) error {
	str, ok := msg.(*string)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected message type: %T", msg)
	}
	if err := c.ServerStream.RecvMsg(str); err != nil {
		return err
	}
	*str += "," + c.id
	return nil
}

type testChainHandler struct {
	t *testing.T
}

func (t *testChainHandler) handleUnary(ctx context.Context, req any) (any, error) {
	str, ok := req.(string)
	if !ok {
		return nil, fmt.Errorf("invalid request: %T", req)
	}
	if str != "req,A,B,C" {
		t.t.Errorf("unexpected request: %s", str)
	}
	headers, _ := metadata.FromIncomingContext(ctx)
	expectHeaders := metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
	if !reflect.DeepEqual(expectHeaders, headers) {
		t.t.Errorf("unexpected headers: %s", headers)
	}
	if err := grpc.SetHeader(ctx, metadata.Pairs("header", "value")); err != nil {
		return nil, err
	}
	if err := grpc.SetTrailer(ctx, metadata.Pairs("trailer", "value")); err != nil {
		return nil, err
	}
	return "reply", nil
}

func (t *testChainHandler) handleStream(_ any, stream grpc.ServerStream) error {
	headers, _ := metadata.FromIncomingContext(stream.Context())
	expectHeaders := metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
	if !reflect.DeepEqual(expectHeaders, headers) {
		t.t.Errorf("unexpected headers: %s", headers)
	}
	var count int
	for {
		var req string
		if err := stream.RecvMsg(&req); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		count++
		if req != "req"+strconv.Itoa(count)+",A,B,C" {
			t.t.Errorf("unexpected request: %s", req)
		}
	}
	if err := stream.SetHeader(metadata.Pairs("header", "value")); err != nil {
		return err
	}
	for i := range count {
		if err := stream.SendMsg("reply" + strconv.Itoa(count-i)); err != nil {
			return err
		}
	}
	stream.SetTrailer(metadata.Pairs("trailer", "value"))
	return nil
}

func (t *testChainHandler) serviceDesc() *grpc.ServiceDesc {
	unaryHandler := t.handleUnary
	streamHandler := t.handleStream
	return &grpc.ServiceDesc{
		ServiceName: "foo",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "bar",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					var req string
					if err := dec(&req); err != nil {
						return nil, err
					}
					if interceptor != nil {
						return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: "/foo/bar"}, unaryHandler)
					}
					return unaryHandler(ctx, req)
				},
			},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName: "bar",
				Handler:    streamHandler,
			},
		},
	}
}

type testChainStreamServer struct {
	t         *testing.T
	ctx       context.Context
	reqSuffix string
	done      bool
	count     int

	headers, trailers metadata.MD
	sent              []string
}

func (t *testChainStreamServer) SetHeader(md metadata.MD) error {
	if t.done {
		return io.EOF
	}
	if t.headers == nil {
		t.headers = metadata.MD{}
	}
	for k, v := range md {
		t.headers[k] = append(t.headers[k], v...)
	}
	return nil
}

func (t *testChainStreamServer) SendHeader(md metadata.MD) error {
	if err := t.SetHeader(md); err != nil {
		return err
	}
	t.done = true
	return nil
}

func (t *testChainStreamServer) SetTrailer(md metadata.MD) {
	if t.trailers == nil {
		t.trailers = metadata.MD{}
	}
	for k, v := range md {
		t.trailers[k] = append(t.trailers[k], v...)
	}
}

func (t *testChainStreamServer) Context() context.Context {
	return t.ctx
}

func (t *testChainStreamServer) SendMsg(m any) error {
	str, ok := m.(string)
	if !ok {
		return fmt.Errorf("invalid message: %T", m)
	}
	t.sent = append(t.sent, str)
	return nil
}

func (t *testChainStreamServer) RecvMsg(m any) error {
	if t.count == 3 {
		return io.EOF
	}
	str, ok := m.(*string)
	if !ok {
		return fmt.Errorf("invalid message: %T", m)
	}
	t.count++
	*str = "req" + strconv.Itoa(t.count) + t.reqSuffix
	return nil
}
