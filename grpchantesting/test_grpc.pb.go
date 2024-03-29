// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package grpchantesting

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// TestServiceClient is the client API for TestService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TestServiceClient interface {
	Unary(ctx context.Context, in *Message, opts ...grpc.CallOption) (*Message, error)
	ClientStream(ctx context.Context, opts ...grpc.CallOption) (TestService_ClientStreamClient, error)
	ServerStream(ctx context.Context, in *Message, opts ...grpc.CallOption) (TestService_ServerStreamClient, error)
	BidiStream(ctx context.Context, opts ...grpc.CallOption) (TestService_BidiStreamClient, error)
	// UseExternalMessageTwice is here purely to test the protoc-gen-grpchan plug-in
	UseExternalMessageTwice(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type testServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTestServiceClient(cc grpc.ClientConnInterface) TestServiceClient {
	return &testServiceClient{cc}
}

func (c *testServiceClient) Unary(ctx context.Context, in *Message, opts ...grpc.CallOption) (*Message, error) {
	out := new(Message)
	err := c.cc.Invoke(ctx, "/grpchantesting.TestService/Unary", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) ClientStream(ctx context.Context, opts ...grpc.CallOption) (TestService_ClientStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[0], "/grpchantesting.TestService/ClientStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceClientStreamClient{stream}
	return x, nil
}

type TestService_ClientStreamClient interface {
	Send(*Message) error
	CloseAndRecv() (*Message, error)
	grpc.ClientStream
}

type testServiceClientStreamClient struct {
	grpc.ClientStream
}

func (x *testServiceClientStreamClient) Send(m *Message) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceClientStreamClient) CloseAndRecv() (*Message, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Message)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) ServerStream(ctx context.Context, in *Message, opts ...grpc.CallOption) (TestService_ServerStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[1], "/grpchantesting.TestService/ServerStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceServerStreamClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TestService_ServerStreamClient interface {
	Recv() (*Message, error)
	grpc.ClientStream
}

type testServiceServerStreamClient struct {
	grpc.ClientStream
}

func (x *testServiceServerStreamClient) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) BidiStream(ctx context.Context, opts ...grpc.CallOption) (TestService_BidiStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestService_ServiceDesc.Streams[2], "/grpchantesting.TestService/BidiStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceBidiStreamClient{stream}
	return x, nil
}

type TestService_BidiStreamClient interface {
	Send(*Message) error
	Recv() (*Message, error)
	grpc.ClientStream
}

type testServiceBidiStreamClient struct {
	grpc.ClientStream
}

func (x *testServiceBidiStreamClient) Send(m *Message) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceBidiStreamClient) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) UseExternalMessageTwice(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/grpchantesting.TestService/UseExternalMessageTwice", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TestServiceServer is the server API for TestService service.
// All implementations must embed UnimplementedTestServiceServer
// for forward compatibility
type TestServiceServer interface {
	Unary(context.Context, *Message) (*Message, error)
	ClientStream(TestService_ClientStreamServer) error
	ServerStream(*Message, TestService_ServerStreamServer) error
	BidiStream(TestService_BidiStreamServer) error
	// UseExternalMessageTwice is here purely to test the protoc-gen-grpchan plug-in
	UseExternalMessageTwice(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	mustEmbedUnimplementedTestServiceServer()
}

// UnimplementedTestServiceServer must be embedded to have forward compatible implementations.
type UnimplementedTestServiceServer struct {
}

func (UnimplementedTestServiceServer) Unary(context.Context, *Message) (*Message, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Unary not implemented")
}
func (UnimplementedTestServiceServer) ClientStream(TestService_ClientStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method ClientStream not implemented")
}
func (UnimplementedTestServiceServer) ServerStream(*Message, TestService_ServerStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method ServerStream not implemented")
}
func (UnimplementedTestServiceServer) BidiStream(TestService_BidiStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method BidiStream not implemented")
}
func (UnimplementedTestServiceServer) UseExternalMessageTwice(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UseExternalMessageTwice not implemented")
}
func (UnimplementedTestServiceServer) mustEmbedUnimplementedTestServiceServer() {}

// UnsafeTestServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TestServiceServer will
// result in compilation errors.
type UnsafeTestServiceServer interface {
	mustEmbedUnimplementedTestServiceServer()
}

func RegisterTestServiceServer(s grpc.ServiceRegistrar, srv TestServiceServer) {
	s.RegisterService(&TestService_ServiceDesc, srv)
}

func _TestService_Unary_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).Unary(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/grpchantesting.TestService/Unary",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).Unary(ctx, req.(*Message))
	}
	return interceptor(ctx, in, info, handler)
}

func _TestService_ClientStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).ClientStream(&testServiceClientStreamServer{stream})
}

type TestService_ClientStreamServer interface {
	SendAndClose(*Message) error
	Recv() (*Message, error)
	grpc.ServerStream
}

type testServiceClientStreamServer struct {
	grpc.ServerStream
}

func (x *testServiceClientStreamServer) SendAndClose(m *Message) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceClientStreamServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TestService_ServerStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Message)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TestServiceServer).ServerStream(m, &testServiceServerStreamServer{stream})
}

type TestService_ServerStreamServer interface {
	Send(*Message) error
	grpc.ServerStream
}

type testServiceServerStreamServer struct {
	grpc.ServerStream
}

func (x *testServiceServerStreamServer) Send(m *Message) error {
	return x.ServerStream.SendMsg(m)
}

func _TestService_BidiStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).BidiStream(&testServiceBidiStreamServer{stream})
}

type TestService_BidiStreamServer interface {
	Send(*Message) error
	Recv() (*Message, error)
	grpc.ServerStream
}

type testServiceBidiStreamServer struct {
	grpc.ServerStream
}

func (x *testServiceBidiStreamServer) Send(m *Message) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceBidiStreamServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TestService_UseExternalMessageTwice_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServiceServer).UseExternalMessageTwice(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/grpchantesting.TestService/UseExternalMessageTwice",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServiceServer).UseExternalMessageTwice(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// TestService_ServiceDesc is the grpc.ServiceDesc for TestService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TestService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpchantesting.TestService",
	HandlerType: (*TestServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Unary",
			Handler:    _TestService_Unary_Handler,
		},
		{
			MethodName: "UseExternalMessageTwice",
			Handler:    _TestService_UseExternalMessageTwice_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ClientStream",
			Handler:       _TestService_ClientStream_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "ServerStream",
			Handler:       _TestService_ServerStream_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "BidiStream",
			Handler:       _TestService_BidiStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "test.proto",
}
