package grpchan_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/internal"
)

func TestInterceptClientConnUnary(t *testing.T) {
	tc := testConn{}

	var successCount, failCount int
	intercepted := grpchan.InterceptClientConn(&tc,
		func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			if err := invoker(ctx, method, req, reply, cc, opts...); err != nil {
				failCount++
				return err
			}
			successCount++
			return nil
		}, nil)

	cli := grpchantesting.NewTestServiceClient(intercepted)

	// success
	tc.resp = &grpchantesting.Message{Count: 123}
	resp, err := cli.Unary(context.Background(), &grpchantesting.Message{})
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}
	if !proto.Equal(resp, tc.resp.(proto.Message)) {
		t.Fatalf("unexpected reply: %v != %v", resp, tc.resp)
	}

	// failure
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("foo", "bar"))
	tc.code = codes.Aborted
	_, err = cli.Unary(ctx, &grpchantesting.Message{Count: 456})
	if err == nil {
		t.Fatalf("expected RPC to fail")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("wrong type of error %T: %v", err, err)
	}
	if s.Code() != codes.Aborted {
		t.Fatalf("wrong error code: %v != %v", s.Code(), codes.Aborted)
	}

	// check observed state
	if successCount != 1 {
		t.Fatalf("interceptor observed wrong number of successful RPCs: expecting %d, got %d", 1, successCount)
	}
	if failCount != 1 {
		t.Fatalf("interceptor observed wrong number of failed RPCs: expecting %d, got %d", 1, failCount)
	}

	expected := []*call{
		{
			methodName: "/grpchantesting.TestService/Unary",
			reqs:       []proto.Message{&grpchantesting.Message{}},
			headers:    nil,
		},
		{
			methodName: "/grpchantesting.TestService/Unary",
			reqs:       []proto.Message{&grpchantesting.Message{Count: 456}},
			headers:    metadata.Pairs("foo", "bar"),
		},
	}

	checkCalls(t, expected, tc.calls)
}

func TestInterceptClientConnStream(t *testing.T) {
	tc := testConn{}

	var messageCount, successCount, failCount int
	intercepted := grpchan.InterceptClientConn(&tc, nil,
		func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			cs, err := streamer(ctx, desc, cc, method, opts...)
			if err != nil {
				return nil, err
			}
			return &testInterceptClientStream{
				ClientStream:  cs,
				messageCount:  &messageCount,
				successCount:  &successCount,
				failCount:     &failCount,
				serverStreams: desc.ServerStreams,
			}, nil
		})

	cli := grpchantesting.NewTestServiceClient(intercepted)

	// client stream, success
	tc.resp = &grpchantesting.Message{Count: 123}
	cs, err := cli.ClientStream(context.Background())
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	err = cs.Send(&grpchantesting.Message{})
	if err != nil {
		t.Fatalf("sending request #1 failed: %v", err)
	}
	err = cs.Send(&grpchantesting.Message{Count: 1})
	if err != nil {
		t.Fatalf("sending request #2 failed: %v", err)
	}
	err = cs.Send(&grpchantesting.Message{Count: 42})
	if err != nil {
		t.Fatalf("sending request #3 failed: %v", err)
	}
	resp, err := cs.CloseAndRecv()
	if err != nil {
		t.Fatalf("failed to receive response: %v", err)
	}
	if !proto.Equal(resp, tc.resp.(proto.Message)) {
		t.Fatalf("unexpected reply: %v != %v", resp, tc.resp)
	}

	// server stream, success
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("foo", "bar"))
	tc.respCount = 5
	ss, err := cli.ServerStream(ctx, &grpchantesting.Message{Count: 456})
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		resp, err = ss.Recv()
		if err != nil {
			t.Fatalf("failed to receive response #%d: %v", i+1, err)
		}
		if !proto.Equal(resp, tc.resp.(proto.Message)) {
			t.Fatalf("unexpected reply #%d: %v != %v", i+1, resp, tc.resp)
		}
	}

	_, err = ss.Recv()
	if err != io.EOF {
		t.Fatalf("expected EOF, instead got %v", err)
	}

	// bidi stream, failure
	ctx = metadata.NewOutgoingContext(context.Background(), metadata.Pairs("foo", "baz"))
	tc.code = codes.Aborted
	bs, err := cli.BidiStream(ctx)
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	err = bs.Send(&grpchantesting.Message{Count: 333})
	if err != nil {
		t.Fatalf("sending request #1 failed: %v", err)
	}
	err = bs.Send(&grpchantesting.Message{Count: 222})
	if err != nil {
		t.Fatalf("sending request #2 failed: %v", err)
	}
	err = bs.Send(&grpchantesting.Message{Count: 111})
	if err != nil {
		t.Fatalf("sending request #3 failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		resp, err = bs.Recv()
		if err != nil {
			t.Fatalf("failed to receive response #%d: %v", i+1, err)
		}
		if !proto.Equal(resp, tc.resp.(proto.Message)) {
			t.Fatalf("unexpected reply #%d: %v != %v", i+1, resp, tc.resp)
		}
	}

	_, err = bs.Recv()
	if err == nil {
		t.Fatalf("expected RPC to fail")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("wrong type of error %T: %v", err, err)
	}
	if s.Code() != codes.Aborted {
		t.Fatalf("wrong error code: %v != %v", s.Code(), codes.Aborted)
	}

	// check observed state
	expectedMessages := 1 + 5 + 5
	if messageCount != expectedMessages {
		t.Fatalf("interceptor observed wrong number of response messages: expecting %d, got %d", expectedMessages, messageCount)
	}
	if successCount != 2 {
		t.Fatalf("interceptor observed wrong number of successful RPCs: expecting %d, got %d", 2, successCount)
	}
	if failCount != 1 {
		t.Fatalf("interceptor observed wrong number of failed RPCs: expecting %d, got %d", 1, failCount)
	}

	expected := []*call{
		{
			methodName: "/grpchantesting.TestService/ClientStream",
			reqs: []proto.Message{
				&grpchantesting.Message{},
				&grpchantesting.Message{Count: 1},
				&grpchantesting.Message{Count: 42},
			},
			headers: nil,
		},
		{
			methodName: "/grpchantesting.TestService/ServerStream",
			reqs:       []proto.Message{&grpchantesting.Message{Count: 456}},
			headers:    metadata.Pairs("foo", "bar"),
		},
		{
			methodName: "/grpchantesting.TestService/BidiStream",
			reqs: []proto.Message{
				&grpchantesting.Message{Count: 333},
				&grpchantesting.Message{Count: 222},
				&grpchantesting.Message{Count: 111},
			},
			headers: metadata.Pairs("foo", "baz"),
		},
	}

	checkCalls(t, expected, tc.calls)
}

type testInterceptClientStream struct {
	grpc.ClientStream
	messageCount, successCount, failCount *int
	serverStreams, closed                 bool
}

func (s *testInterceptClientStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if err == nil {
		*s.messageCount++
		if !s.serverStreams {
			s.closed = true
			*s.successCount++
		}
	} else if !s.closed {
		s.closed = true
		if err == io.EOF {
			*s.successCount++
		} else {
			*s.failCount++
		}
	}
	return err
}

// testConn is a dummy channel that just records all incoming activity.
//
// If code is set and not codes.OK, RPCs will fail with that code.
//
// If resp is set, unary RPCs will reply with that value. If unset, unary
// RPCs will reply with empty response message.
//
// If resp is set and respCount is non-zero, server-streaming RPCs (including
// bidi streams) will reply with the given number of responses. Otherwise,
// they reply with an empty stream.
//
// Streaming RPCs will receive the specified headers and trailers as response
// metadata, if those fields are set.
//
// testConn is not thread-safe, and neither are any returned streams.
type testConn struct {
	code      codes.Code
	resp      interface{}
	respCount int
	headers   metadata.MD
	trailers  metadata.MD
	calls     []*call
}

type call struct {
	methodName string
	headers    metadata.MD
	reqs       []proto.Message
}

func (ch *testConn) Invoke(ctx context.Context, methodName string, req, resp interface{}, _ ...grpc.CallOption) error {
	headers, _ := metadata.FromOutgoingContext(ctx)
	reqClone, err := internal.CloneMessage(req)
	if err != nil {
		return err
	}
	ch.calls = append(ch.calls, &call{methodName: methodName, headers: headers, reqs: []proto.Message{reqClone.(proto.Message)}})
	if ch.code != codes.OK {
		return status.Error(ch.code, ch.code.String())
	}
	if ch.resp != nil {
		return internal.CopyMessage(resp, ch.resp)
	}
	return internal.ClearMessage(resp)
}

func (ch *testConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
	headers, _ := metadata.FromOutgoingContext(ctx)
	call := &call{methodName: methodName, headers: headers}
	ch.calls = append(ch.calls, call)
	count := ch.respCount
	if !desc.ServerStreams {
		if ch.code == codes.OK {
			count = 1
		} else {
			count = 0
		}
	}
	return &testClientStream{
		ctx:       ctx,
		code:      ch.code,
		resp:      ch.resp,
		respCount: count,
		headers:   ch.headers,
		trailers:  ch.trailers,
		call:      call,
	}, nil
}

type testClientStream struct {
	ctx        context.Context
	code       codes.Code
	resp       interface{}
	respCount  int
	headers    metadata.MD
	trailers   metadata.MD
	call       *call
	halfClosed bool
	closed     bool
}

func (s *testClientStream) Header() (metadata.MD, error) {
	return s.headers, nil
}

func (s *testClientStream) Trailer() metadata.MD {
	return s.trailers
}

func (s *testClientStream) CloseSend() error {
	s.halfClosed = true
	return nil
}

func (s *testClientStream) Context() context.Context {
	return s.ctx
}

func (s *testClientStream) SendMsg(m interface{}) error {
	if s.halfClosed {
		return fmt.Errorf("stream closed")
	}
	if s.closed {
		return io.EOF
	}
	if err := s.ctx.Err(); err != nil {
		return internal.TranslateContextError(err)
	}
	mClone, err := internal.CloneMessage(m)
	if err != nil {
		return err
	}
	s.call.reqs = append(s.call.reqs, mClone.(proto.Message))
	return nil
}

func (s *testClientStream) RecvMsg(m interface{}) error {
	if err := s.ctx.Err(); err != nil {
		return internal.TranslateContextError(err)
	}
	if s.respCount == 0 {
		s.closed = true
		if s.code == codes.OK {
			return io.EOF
		} else {
			return status.Error(s.code, s.code.String())
		}
	}

	s.respCount--
	if s.resp != nil {
		return internal.CopyMessage(m, s.resp)
	}
	return internal.ClearMessage(m)
}
