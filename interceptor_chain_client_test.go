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

var interceptorChainCases = []struct {
	name              string
	setupClient       func(grpc.ClientConnInterface) grpc.ClientConnInterface
	setupServer       func(grpc.ServiceRegistrar) grpc.ServiceRegistrar
	unaryIntercepted  bool
	streamIntercepted bool
}{
	{
		name: "batch",
		setupClient: func(conn grpc.ClientConnInterface) grpc.ClientConnInterface {
			return setupClientChainBatch(conn)
		},
		unaryIntercepted:  true,
		streamIntercepted: true,
	},
	{
		name: "singles",
		setupClient: func(conn grpc.ClientConnInterface) grpc.ClientConnInterface {
			return setupClientChainSingles(conn)
		},
		unaryIntercepted:  true,
		streamIntercepted: true,
	},
	{
		name: "pairs",
		setupClient: func(conn grpc.ClientConnInterface) grpc.ClientConnInterface {
			return setupClientChainPairs(conn)
		},
		unaryIntercepted:  true,
		streamIntercepted: true,
	},
	{
		name: "unary-only",
		setupClient: func(conn grpc.ClientConnInterface) grpc.ClientConnInterface {
			return setupClientChainUnaryOnly(conn)
		},
		unaryIntercepted:  true,
		streamIntercepted: false,
	},
	{
		name: "stream-only",
		setupClient: func(conn grpc.ClientConnInterface) grpc.ClientConnInterface {
			return setupClientChainStreamOnly(conn)
		},
		unaryIntercepted:  false,
		streamIntercepted: true,
	},
	{
		name: "none",
		setupClient: func(conn grpc.ClientConnInterface) grpc.ClientConnInterface {
			return conn
		},
		unaryIntercepted:  false,
		streamIntercepted: false,
	},
}

func TestInterceptorChainClient_Unary(t *testing.T) {
	for _, testCase := range interceptorChainCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			tc := testChainConn{t: t}
			intercepted := testCase.setupClient(&tc)
			var req, expectReply string
			var expectHeaders, expectTrailers metadata.MD
			if testCase.unaryIntercepted {
				req = "req"
				ctx = metadata.AppendToOutgoingContext(ctx, "header", "value")
				expectHeaders = metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
				expectTrailers = metadata.Pairs("trailer", "value", "trailer-A", "value-A", "trailer-B", "value-B", "trailer-C", "value-C")
				expectReply = "reply,C,B,A"
			} else {
				// Need to add stuff to the request and headers, etc that would otherwise be added by
				// interceptors for the test channel to be happy.
				req = "req,A,B,C"
				ctx = metadata.AppendToOutgoingContext(ctx, "header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
				// And we don't expect anything to be added to the response stuff.
				expectHeaders = metadata.Pairs("header", "value")
				expectTrailers = metadata.Pairs("trailer", "value")
				expectReply = "reply"
			}
			var reply string
			var headers, trailers metadata.MD
			if err := intercepted.Invoke(ctx, "/foo/bar", req, &reply, grpc.Header(&headers), grpc.Trailer(&trailers)); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			} else if reply != expectReply {
				t.Errorf("unexpected reply: %s", reply)
			}
			if !reflect.DeepEqual(expectHeaders, headers) {
				t.Errorf("unexpected headers: %s", headers)
			}
			if !reflect.DeepEqual(expectTrailers, trailers) {
				t.Errorf("unexpected trailers: %s", trailers)
			}
		})
	}
}

func TestInterceptorChainClient_Stream(t *testing.T) {
	for _, testCase := range interceptorChainCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			tc := testChainConn{t: t}
			intercepted := testCase.setupClient(&tc)
			var reqSuffix, expectReplySuffix string
			var expectHeaders, expectTrailers metadata.MD
			if testCase.streamIntercepted {
				reqSuffix = ""
				ctx = metadata.AppendToOutgoingContext(ctx, "header", "value")
				expectHeaders = metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
				expectTrailers = metadata.Pairs("trailer", "value", "trailer-A", "value-A", "trailer-B", "value-B", "trailer-C", "value-C")
				expectReplySuffix = ",C,B,A"
			} else {
				// Need to add stuff to the request and headers, etc that would otherwise be added by
				// interceptors for the test channel to be happy.
				reqSuffix = ",A,B,C"
				ctx = metadata.AppendToOutgoingContext(ctx, "header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
				// And we don't expect anything to be added to the response stuff.
				expectHeaders = metadata.Pairs("header", "value")
				expectTrailers = metadata.Pairs("trailer", "value")
				expectReplySuffix = ""
			}

			desc := &grpc.StreamDesc{StreamName: "bar", ClientStreams: true, ServerStreams: true}
			stream, err := intercepted.NewStream(ctx, desc, "/foo/bar")
			if err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if err := stream.SendMsg("req1" + reqSuffix); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if err := stream.SendMsg("req2" + reqSuffix); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if err := stream.SendMsg("req3" + reqSuffix); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if err := stream.CloseSend(); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}

			var reply string
			if err := stream.RecvMsg(&reply); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if reply != "reply3"+expectReplySuffix {
				t.Errorf("unexpected reply: %s", reply)
			}
			if err := stream.RecvMsg(&reply); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if reply != "reply2"+expectReplySuffix {
				t.Errorf("unexpected reply: %s", reply)
			}
			if err := stream.RecvMsg(&reply); err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if reply != "reply1"+expectReplySuffix {
				t.Errorf("unexpected reply: %s", reply)
			}
			if err := stream.RecvMsg(&reply); err == nil {
				t.Error("expecting io.EOF but got no error")
			} else if !errors.Is(err, io.EOF) {
				t.Errorf("expecting io.EOF but got %v", err)
			}

			headers, err := stream.Header()
			if err != nil {
				t.Errorf("unexpected RPC error: %v", err)
			}
			if !reflect.DeepEqual(expectHeaders, headers) {
				t.Errorf("unexpected headers: %s", headers)
			}
			trailers := stream.Trailer()
			if !reflect.DeepEqual(expectTrailers, trailers) {
				t.Errorf("unexpected trailers: %s", trailers)
			}
		})
	}
}

func setupClientChainBatch(clientConn grpc.ClientConnInterface) grpc.ClientConnInterface {
	int1 := chainClientInterceptor{id: "A"}
	int2 := chainClientInterceptor{id: "B"}
	int3 := chainClientInterceptor{id: "C"}
	return InterceptClientConnUnary(
		InterceptClientConnStream(
			clientConn,
			int1.doStream, int2.doStream, int3.doStream,
		),
		int1.doUnary, int2.doUnary, int3.doUnary,
	)
}

func setupClientChainSingles(clientConn grpc.ClientConnInterface) grpc.ClientConnInterface {
	int1 := chainClientInterceptor{id: "A"}
	int2 := chainClientInterceptor{id: "B"}
	int3 := chainClientInterceptor{id: "C"}
	return InterceptClientConnStream(
		InterceptClientConnUnary(
			InterceptClientConnStream(
				InterceptClientConnUnary(
					InterceptClientConnStream(
						InterceptClientConnUnary(
							clientConn,
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

func setupClientChainPairs(clientConn grpc.ClientConnInterface) grpc.ClientConnInterface {
	int1 := chainClientInterceptor{id: "A"}
	int2 := chainClientInterceptor{id: "B"}
	int3 := chainClientInterceptor{id: "C"}
	return InterceptClientConn(
		InterceptClientConn(
			InterceptClientConn(
				clientConn,
				int3.doUnary, int3.doStream,
			),
			int2.doUnary, int2.doStream,
		),
		int1.doUnary, int1.doStream,
	)
}

func setupClientChainUnaryOnly(clientConn grpc.ClientConnInterface) grpc.ClientConnInterface {
	int1 := chainClientInterceptor{id: "A"}
	int2 := chainClientInterceptor{id: "B"}
	int3 := chainClientInterceptor{id: "C"}
	return InterceptClientConnUnary(
		InterceptClientConnUnary(
			InterceptClientConnUnary(
				clientConn,
				int3.doUnary,
			),
			int2.doUnary,
		),
		int1.doUnary,
	)
}

func setupClientChainStreamOnly(clientConn grpc.ClientConnInterface) grpc.ClientConnInterface {
	int1 := chainClientInterceptor{id: "A"}
	int2 := chainClientInterceptor{id: "B"}
	int3 := chainClientInterceptor{id: "C"}
	return InterceptClientConnStream(
		InterceptClientConnStream(
			InterceptClientConnStream(
				clientConn,
				int3.doStream,
			),
			int2.doStream,
		),
		int1.doStream,
	)
}

type chainClientInterceptor struct {
	id string
}

func (c *chainClientInterceptor) doUnary(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "header-"+c.id, "value-"+c.id)
	str, ok := req.(string)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected request type: %T", req)
	}
	str += "," + c.id
	strPtr, ok := reply.(*string)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected response type: %T", reply)
	}
	var headers, trailers metadata.MD
	opts = append(opts, grpc.Header(&headers), grpc.Trailer(&trailers))
	if err := invoker(ctx, method, str, strPtr, cc, opts...); err != nil {
		return err
	}
	if headers == nil {
		return errors.New("response headers are nil")
	}
	headers.Append("header-"+c.id, "value-"+c.id)
	if trailers == nil {
		return errors.New("response trailers are nil")
	}
	trailers.Append("trailer-"+c.id, "value-"+c.id)
	*strPtr += "," + c.id
	return nil
}

func (c *chainClientInterceptor) doStream(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "header-"+c.id, "value-"+c.id)
	stream, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, err
	}
	return &chainClientStream{ClientStream: stream, id: c.id}, nil
}

type chainClientStream struct {
	grpc.ClientStream
	id string
}

func (c *chainClientStream) Header() (metadata.MD, error) {
	md, err := c.ClientStream.Header()
	if err != nil {
		return nil, err
	}
	if md == nil {
		return nil, errors.New("response header is nil")
	}
	md.Append("header-"+c.id, "value-"+c.id)
	return md, nil
}

func (c *chainClientStream) Trailer() metadata.MD {
	md := c.ClientStream.Trailer()
	if md != nil {
		md.Append("trailer-"+c.id, "value-"+c.id)
	}
	return md
}

func (c *chainClientStream) SendMsg(msg any) error {
	str, ok := msg.(string)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected message type: %T", msg)
	}
	str += "," + c.id
	return c.ClientStream.SendMsg(str)
}

func (c *chainClientStream) RecvMsg(msg any) error {
	str, ok := msg.(*string)
	if !ok {
		return status.Errorf(codes.Internal, "unexpected message type: %T", msg)
	}
	if err := c.ClientStream.RecvMsg(str); err != nil {
		return err
	}
	*str += "," + c.id
	return nil
}

type testChainConn struct {
	t *testing.T
}

func (t *testChainConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	if method != "/foo/bar" {
		return fmt.Errorf("unexpected method: %s", method)
	}
	str, ok := args.(string)
	if !ok {
		return fmt.Errorf("invalid request: %T", args)
	}
	if str != "req,A,B,C" {
		t.t.Errorf("unexpected request: %s", str)
	}
	headers, _ := metadata.FromOutgoingContext(ctx)
	expectHeaders := metadata.Pairs("header", "value", "header-A", "value-A", "header-B", "value-B", "header-C", "value-C")
	if !reflect.DeepEqual(expectHeaders, headers) {
		t.t.Errorf("unexpected headers: %s", headers)
	}
	strPtr, ok := reply.(*string)
	if !ok {
		return fmt.Errorf("invalid response: %T", args)
	}
	*strPtr = "reply"
	callOpts := internal.GetCallOptions(opts)
	callOpts.SetHeaders(metadata.Pairs("header", "value"))
	callOpts.SetTrailers(metadata.Pairs("trailer", "value"))
	return nil
}

func (t *testChainConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return &testChainStreamClient{t: t.t, ctx: ctx}, nil
}

type testChainStreamClient struct {
	t     *testing.T
	ctx   context.Context
	count int
	done  bool
}

func (t *testChainStreamClient) Header() (metadata.MD, error) {
	return metadata.Pairs("header", "value"), nil
}

func (t *testChainStreamClient) Trailer() metadata.MD {
	return metadata.Pairs("trailer", "value")
}

func (t *testChainStreamClient) CloseSend() error {
	t.done = true
	return nil
}

func (t *testChainStreamClient) Context() context.Context {
	return t.ctx
}

func (t *testChainStreamClient) SendMsg(m any) error {
	if t.done {
		return io.EOF
	}
	t.count++
	str, ok := m.(string)
	if !ok {
		return fmt.Errorf("invalid message: %T", m)
	}
	if str != "req"+strconv.Itoa(t.count)+",A,B,C" {
		t.t.Errorf("unexpected request: %s", str)
	}
	return nil

}

func (t *testChainStreamClient) RecvMsg(m any) error {
	if t.count == 0 {
		t.done = true
		return io.EOF
	}
	str, ok := m.(*string)
	if !ok {
		return fmt.Errorf("invalid message: %T", m)
	}
	*str = "reply" + strconv.Itoa(t.count)
	t.count--
	return nil
}
