package grpchan_test

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/internal"
)

func TestInterceptServerUnary(t *testing.T) {
	svr := &testServer{}
	handlers := grpchan.HandlerMap{}

	// this will make sure unary interceptors are composed correctly
	var lastSeen string
	outerInt := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		lastSeen = "a"
		return handler(ctx, req)
	}

	var successCount, failCount int
	grpchantesting.RegisterTestServiceHandler(grpchan.WithInterceptor(handlers,
		func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			if lastSeen != "a" {
				// interceptor above should have been invoked first!
				return nil, fmt.Errorf("interceptor not correctly invoked!")
			}
			lastSeen = "b"
			resp, err := handler(ctx, req)
			if err != nil {
				failCount++
			} else {
				successCount++
			}
			return resp, err
		}, nil), svr)

	sd, ss := handlers.QueryService("grpchantesting.TestService")
	// sanity check
	if ss != svr {
		t.Fatalf("queried handler does not match registered handler! %v != %v", ss, svr)
	}
	if sd == nil {
		t.Fatalf("service descriptor not found")
	}

	// get handler for the method we're going to invoke
	md := internal.FindUnaryMethod("Unary", sd.Methods)
	if md == nil {
		t.Fatalf("method descriptor not found")
	}

	// success
	svr.resp = &grpchantesting.Message{Count: 123}
	var m grpchantesting.Message
	dec := func(req interface{}) error {
		*(req.(*grpchantesting.Message)) = m
		return nil
	}
	resp, err := md.Handler(svr, context.Background(), dec, outerInt)
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}
	if !reflect.DeepEqual(resp, svr.resp) {
		t.Fatalf("unexpected reply: expecting %v; got %v", svr.resp, resp)
	}
	if lastSeen != "b" {
		t.Fatalf("interceptors not composed correctly")
	}

	// failure
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("foo", "bar"))
	svr.code = codes.Aborted
	m = grpchantesting.Message{Count: 456}
	_, err = md.Handler(svr, ctx, dec, outerInt)
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
	if lastSeen != "b" {
		t.Fatalf("interceptors not composed correctly")
	}

	// check observed state
	if successCount != 1 {
		t.Fatalf("interceptor observed wrong number of successful RPCs: expecting %d, got %d", 1, successCount)
	}
	if failCount != 1 {
		t.Fatalf("interceptor observed wrong number of failed RPCs: expecting %d, got %d", 1, failCount)
	}

	expected := []interface{}{
		&unaryCall{
			methodName: "Unary",
			req:        &grpchantesting.Message{},
			headers:    nil,
		},
		&unaryCall{
			methodName: "Unary",
			req:        &grpchantesting.Message{Count: 456},
			headers:    metadata.Pairs("foo", "bar"),
		},
	}

	if !reflect.DeepEqual(svr.calls, expected) {
		t.Fatalf("unexpected calls observed by server: expecting %v; got %v", expected, svr.calls)
	}
}

func TestInterceptServerStream(t *testing.T) {
	svr := &testServer{}
	handlers := grpchan.HandlerMap{}

	var messageCount, successCount, failCount int
	grpchantesting.RegisterTestServiceHandler(grpchan.WithInterceptor(handlers, nil,
		func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			err := handler(srv, &testInterceptServerStream{
				ServerStream: ss,
				messageCount: &messageCount,
			})
			if err != nil {
				failCount++
			} else {
				successCount++
			}
			return err
		}), svr)

	sd, ss := handlers.QueryService("grpchantesting.TestService")
	// sanity check
	if ss != svr {
		t.Fatalf("queried handler does not match registered handler! %v != %v", ss, svr)
	}
	if sd == nil {
		t.Fatalf("service descriptor not found")
	}

	// get handlers for the methods we're going to invoke
	csdesc := internal.FindStreamingMethod("ClientStream", sd.Streams)
	if csdesc == nil {
		t.Fatalf("ClientStream stream descriptor not found")
	}
	ssdesc := internal.FindStreamingMethod("ServerStream", sd.Streams)
	if ssdesc == nil {
		t.Fatalf("ServerStream stream descriptor not found")
	}
	bsdesc := internal.FindStreamingMethod("BidiStream", sd.Streams)
	if bsdesc == nil {
		t.Fatalf("BidiStream stream descriptor not found")
	}

	// client stream, success
	svr.resp = &grpchantesting.Message{Count: 123}
	str := &testServerStream{
		ctx: context.Background(),
		reqs: []interface{}{
			&grpchantesting.Message{},
			&grpchantesting.Message{Count: 1},
			&grpchantesting.Message{Count: 42},
		},
	}
	err := csdesc.Handler(svr, str)
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	replies := []interface{}{svr.resp}
	if !reflect.DeepEqual(replies, str.resps) {
		t.Fatalf("unexpected reply: expecting %v; got %v", replies, str.resps)
	}

	// server stream, success
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("foo", "bar"))
	svr.respCount = 5
	str = &testServerStream{
		ctx: ctx,
		reqs: []interface{}{
			&grpchantesting.Message{Count: 456},
		},
	}
	err = ssdesc.Handler(svr, str)
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	replies = []interface{}{svr.resp, svr.resp, svr.resp, svr.resp, svr.resp} // five of 'em
	if !reflect.DeepEqual(replies, str.resps) {
		t.Fatalf("unexpected reply: expecting %v; got %v", replies, str.resps)
	}

	// bidi stream, failure
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("foo", "baz"))
	svr.code = codes.Aborted
	str = &testServerStream{
		ctx: ctx,
		reqs: []interface{}{
			&grpchantesting.Message{Count: 333},
			&grpchantesting.Message{Count: 222},
			&grpchantesting.Message{Count: 111},
		},
	}
	err = bsdesc.Handler(svr, str)
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

	if !reflect.DeepEqual(replies, str.resps) {
		t.Fatalf("unexpected reply: expecting %v; got %v", replies, str.resps)
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

	expected := []interface{}{
		&streamCall{
			methodName: "ClientStream",
			reqs: []interface{}{
				&grpchantesting.Message{},
				&grpchantesting.Message{Count: 1},
				&grpchantesting.Message{Count: 42},
			},
			headers: nil,
		},
		&streamCall{
			methodName: "ServerStream",
			reqs:       []interface{}{&grpchantesting.Message{Count: 456}},
			headers:    metadata.Pairs("foo", "bar"),
		},
		&streamCall{
			methodName: "BidiStream",
			reqs: []interface{}{
				&grpchantesting.Message{Count: 333},
				&grpchantesting.Message{Count: 222},
				&grpchantesting.Message{Count: 111},
			},
			headers: metadata.Pairs("foo", "baz"),
		},
	}

	if !reflect.DeepEqual(svr.calls, expected) {
		t.Fatalf("unexpected calls observed by server: expecting %v; got %v", expected, svr.calls)
	}
}

type testInterceptServerStream struct {
	grpc.ServerStream
	messageCount *int
}

func (s *testInterceptServerStream) SendMsg(m interface{}) error {
	err := s.ServerStream.SendMsg(m)
	if err == nil {
		(*s.messageCount)++
	}
	return err
}

// testServer is a dummy server that just records all incoming activity.
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
// testServer is not thread-safe.
type testServer struct {
	code      codes.Code
	resp      interface{}
	respCount int
	headers   metadata.MD
	trailers  metadata.MD
	calls     []interface{} // elements are either *unaryCall or *streamCall
}

func (s *testServer) Unary(ctx context.Context, req *grpchantesting.Message) (*grpchantesting.Message, error) {
	resp := grpchantesting.Message{}
	err := s.unary(ctx, "Unary", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *testServer) ClientStream(stream grpchantesting.TestService_ClientStreamServer) error {
	return s.stream(&grpc.StreamDesc{
		StreamName:    "ClientStream",
		ClientStreams: true,
	}, nil, stream)
}

func (s *testServer) ServerStream(req *grpchantesting.Message, stream grpchantesting.TestService_ServerStreamServer) error {
	return s.stream(&grpc.StreamDesc{
		StreamName:    "ServerStream",
		ServerStreams: true,
	}, req, stream)
}

func (s *testServer) BidiStream(stream grpchantesting.TestService_BidiStreamServer) error {
	return s.stream(&grpc.StreamDesc{
		StreamName:    "BidiStream",
		ClientStreams: true,
		ServerStreams: true,
	}, nil, stream)
}

func (s *testServer) UseExternalMessageTwice(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	resp := empty.Empty{}
	err := s.unary(ctx, "UseExternalMessageTwice", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *testServer) unary(ctx context.Context, methodName string, req, resp interface{}) error {
	headers, _ := metadata.FromIncomingContext(ctx)
	s.calls = append(s.calls, &unaryCall{methodName: methodName, headers: headers, req: internal.CloneMessage(req)})
	if s.code != codes.OK {
		return status.Error(s.code, s.code.String())
	}
	if s.resp != nil {
		return internal.CopyMessage(s.resp, resp)
	}
	return internal.ClearMessage(resp)
}

func (s *testServer) stream(desc *grpc.StreamDesc, req *grpchantesting.Message, stream grpc.ServerStream) error {
	headers, _ := metadata.FromIncomingContext(stream.Context())
	call := &streamCall{methodName: desc.StreamName, headers: headers}
	s.calls = append(s.calls, call)

	// consume requests
	if desc.ClientStreams {
		for {
			m := &grpchantesting.Message{}
			err := stream.RecvMsg(m)
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			call.reqs = append(call.reqs, m)
		}
	} else {
		call.reqs = append(call.reqs, req)
	}

	// produce responses
	if len(s.headers) > 0 {
		if err := stream.SetHeader(s.headers); err != nil {
			return err
		}
	}

	count := s.respCount
	if !desc.ServerStreams {
		if s.code == codes.OK {
			count = 1
		} else {
			count = 0
		}
	}
	for count > 0 {
		m := s.resp
		if m == nil {
			m = &grpchantesting.Message{}
		}
		if err := stream.SendMsg(m); err != nil {
			return err
		}
		count--
	}

	if len(s.trailers) > 0 {
		stream.SetTrailer(s.trailers)
	}

	if s.code != codes.OK {
		return status.Error(s.code, s.code.String())
	}
	return nil
}

type testServerStream struct {
	ctx         context.Context
	reqs        []interface{}
	resps       []interface{}
	headers     metadata.MD
	headersSent bool
	trailers    metadata.MD
}

func (s *testServerStream) SetHeader(md metadata.MD) error {
	if s.headersSent {
		return fmt.Errorf("headers already sent")
	}
	s.headers = metadata.Join(s.headers, md)
	return nil
}

func (s *testServerStream) SendHeader(md metadata.MD) error {
	if err := s.SetHeader(md); err != nil {
		return err
	}
	s.headersSent = true
	return nil
}

func (s *testServerStream) SetTrailer(md metadata.MD) {
	s.trailers = metadata.Join(s.trailers, md)
}

func (s *testServerStream) Context() context.Context {
	return s.ctx
}

func (s *testServerStream) SendMsg(m interface{}) error {
	if err := s.ctx.Err(); err != nil {
		return internal.TranslateContextError(err)
	}
	s.resps = append(s.resps, internal.CloneMessage(m))
	return nil
}

func (s *testServerStream) RecvMsg(m interface{}) error {
	if len(s.reqs) == 0 {
		return io.EOF
	}
	req := s.reqs[0]
	s.reqs = s.reqs[1:]
	if req != nil {
		return internal.CopyMessage(req, m)
	}
	return internal.ClearMessage(m)
}
