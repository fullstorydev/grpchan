package grpchan

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
)

// WrappedClientConn is a channel that wraps another. It provides an Unwrap method
// for access the underlying wrapped implementation.
type WrappedClientConn interface {
	grpc.ClientConnInterface
	Unwrap() grpc.ClientConnInterface
}

// InterceptChannel returns a new channel that intercepts RPCs with the given
// interceptors. If both given interceptors are nil, returns ch. Otherwise, the
// returned value will implement WrappedClientConn and its Unwrap() method will
// return ch.
//
// Deprecated: Use InterceptClientConn instead.
func InterceptChannel(ch grpc.ClientConnInterface, unaryInt grpc.UnaryClientInterceptor, streamInt grpc.StreamClientInterceptor) grpc.ClientConnInterface {
	return InterceptClientConn(ch, unaryInt, streamInt)
}

// InterceptClientConn returns a new channel that intercepts RPCs with the given
// interceptors, which may be nil. If both given interceptors are nil, returns ch.
// Otherwise, the returned value will implement WrappedClientConn and its Unwrap()
// method will return ch.
func InterceptClientConn(ch grpc.ClientConnInterface, unaryInt grpc.UnaryClientInterceptor, streamInt grpc.StreamClientInterceptor) grpc.ClientConnInterface {
	if unaryInt != nil {
		ch = InterceptClientConnUnary(ch, unaryInt)
	}
	if streamInt != nil {
		ch = InterceptClientConnStream(ch, streamInt)
	}
	return ch
}

// InterceptClientConnUnary returns a new channel that intercepts unary RPCs
// with the given chain of interceptors. If the given set of interceptors is
// empty, this returns ch. Otherwise, the returned value will implement
// WrappedClientConn and its Unwrap() method will return ch.
//
// The first interceptor in the set will be the first one invoked when an RPC
// is called. When that interceptor delegates to the provided invoker, it will
// call the second interceptor, and so on.
func InterceptClientConnUnary(ch grpc.ClientConnInterface, unaryInt ...grpc.UnaryClientInterceptor) grpc.ClientConnInterface {
	if len(unaryInt) == 0 {
		return ch
	}
	var streamInt grpc.StreamClientInterceptor
	intCh, ok := ch.(*interceptedChannel)
	if ok {
		// Instead of building a chain of multiple interceptedChannels, build
		// a single interceptedChannel with the combined set of interceptors.
		ch = intCh.ch
		if intCh.unaryInt != nil {
			unaryInt = append(unaryInt, intCh.unaryInt)
		}
		streamInt = intCh.streamInt
	}
	return &interceptedChannel{ch: ch, unaryInt: chainUnaryClient(unaryInt), streamInt: streamInt}
}

// InterceptClientConnStream returns a new channel that intercepts streaming
// RPCs with the given chain of interceptors. If the given set of interceptors
// is empty, this returns ch. Otherwise, the returned value will implement
// WrappedClientConn and its Unwrap() method will return ch.
//
// The first interceptor in the set will be the first one invoked when an RPC
// is called. When that interceptor delegates to the provided invoker, it will
// call the second interceptor, and so on.
func InterceptClientConnStream(ch grpc.ClientConnInterface, streamInt ...grpc.StreamClientInterceptor) grpc.ClientConnInterface {
	if len(streamInt) == 0 {
		return ch
	}
	var unaryInt grpc.UnaryClientInterceptor
	intCh, ok := ch.(*interceptedChannel)
	if ok {
		// Instead of building a chain of multiple interceptedChannels, build
		// a single interceptedChannel with the combined set of interceptors.
		ch = intCh.ch
		unaryInt = intCh.unaryInt
		if intCh.streamInt != nil {
			streamInt = append(streamInt, intCh.streamInt)
		}
	}
	return &interceptedChannel{ch: ch, unaryInt: unaryInt, streamInt: chainStreamClient(streamInt)}
}

type interceptedChannel struct {
	ch        grpc.ClientConnInterface
	unaryInt  grpc.UnaryClientInterceptor
	streamInt grpc.StreamClientInterceptor
}

func (intch *interceptedChannel) Unwrap() grpc.ClientConnInterface {
	return intch.ch
}

func unwrap(ch grpc.ClientConnInterface) grpc.ClientConnInterface {
	// completely unwrap to find the root ClientConn
	for {
		w, ok := ch.(WrappedClientConn)
		if !ok {
			return ch
		}
		unwrapped := w.Unwrap()
		if unwrapped == nil {
			return ch
		}
		ch = unwrapped
	}
}

func (intch *interceptedChannel) Invoke(ctx context.Context, methodName string, req, resp any, opts ...grpc.CallOption) error {
	if intch.unaryInt == nil {
		return intch.ch.Invoke(ctx, methodName, req, resp, opts...)
	}
	cc, _ := unwrap(intch.ch).(*grpc.ClientConn)
	return intch.unaryInt(ctx, methodName, req, resp, cc, intch.unaryInvoker, opts...)
}

func (intch *interceptedChannel) unaryInvoker(ctx context.Context, methodName string, req, resp any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
	return intch.ch.Invoke(ctx, methodName, req, resp, opts...)
}

func (intch *interceptedChannel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if intch.streamInt == nil {
		return intch.ch.NewStream(ctx, desc, methodName, opts...)
	}
	cc, _ := intch.ch.(*grpc.ClientConn)
	return intch.streamInt(ctx, desc, cc, methodName, intch.streamer, opts...)
}

func (intch *interceptedChannel) streamer(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return intch.ch.NewStream(ctx, desc, methodName, opts...)
}

var _ grpc.ClientConnInterface = (*interceptedChannel)(nil)

func chainUnaryClient(unaryInt []grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	if len(unaryInt) == 1 {
		return unaryInt[0]
	}
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		for i := range unaryInt {
			currInterceptor := unaryInt[len(unaryInt)-i-1] // going backwards through the chain
			currInvoker := invoker
			invoker = func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return currInterceptor(ctx, method, req, reply, cc, currInvoker, opts...)
			}
		}
		return unaryInt[0](ctx, method, req, reply, cc, invoker, opts...)
	}
}

func chainStreamClient(streamInt []grpc.StreamClientInterceptor) grpc.StreamClientInterceptor {
	if len(streamInt) == 1 {
		return streamInt[0]
	}
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		for i := range streamInt {
			currInterceptor := streamInt[len(streamInt)-i-1] // going backwards through the chain
			currStreamer := streamer
			streamer = func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				return currInterceptor(ctx, desc, cc, method, currStreamer, opts...)
			}
		}
		return streamInt[0](ctx, desc, cc, method, streamer, opts...)
	}
}

// WithInterceptor returns a view of the given ServiceRegistrar that will
// automatically apply the given interceptors to all registered services.
func WithInterceptor(reg grpc.ServiceRegistrar, unaryInt grpc.UnaryServerInterceptor, streamInt grpc.StreamServerInterceptor) grpc.ServiceRegistrar {
	if unaryInt != nil {
		reg = WithUnaryInterceptors(reg, unaryInt)
	}
	if streamInt != nil {
		reg = WithStreamInterceptors(reg, streamInt)
	}
	return reg
}

// WithUnaryInterceptors returns a view of the given ServiceRegistrar that will
// automatically apply the given interceptors to all registered services.
func WithUnaryInterceptors(reg grpc.ServiceRegistrar, unaryInt ...grpc.UnaryServerInterceptor) grpc.ServiceRegistrar {
	if len(unaryInt) == 0 {
		return reg
	}
	var streamInt grpc.StreamServerInterceptor
	intReg, ok := reg.(*interceptingRegistry)
	if ok {
		// Instead of building a chain of multiple interceptingRegistry instances,
		// build a single interceptingRegistry with the combined set of interceptors.
		reg = intReg.reg
		if intReg.unaryInt != nil {
			unaryInt = append(unaryInt, intReg.unaryInt)
		}
		streamInt = intReg.streamInt
	}
	return &interceptingRegistry{reg: reg, unaryInt: chainUnaryServer(unaryInt), streamInt: streamInt}
}

func WithStreamInterceptors(reg grpc.ServiceRegistrar, streamInt ...grpc.StreamServerInterceptor) grpc.ServiceRegistrar {
	if len(streamInt) == 0 {
		return reg
	}
	var unaryInt grpc.UnaryServerInterceptor
	intReg, ok := reg.(*interceptingRegistry)
	if ok {
		// Instead of building a chain of multiple interceptingRegistry instances,
		// build a single interceptingRegistry with the combined set of interceptors.
		reg = intReg.reg
		unaryInt = intReg.unaryInt
		if intReg.streamInt != nil {
			streamInt = append(streamInt, intReg.streamInt)
		}
	}
	return &interceptingRegistry{reg: reg, unaryInt: unaryInt, streamInt: chainStreamServer(streamInt)}
}

type interceptingRegistry struct {
	reg       grpc.ServiceRegistrar
	unaryInt  grpc.UnaryServerInterceptor
	streamInt grpc.StreamServerInterceptor
}

func (r *interceptingRegistry) RegisterService(desc *grpc.ServiceDesc, srv any) {
	r.reg.RegisterService(InterceptServer(desc, r.unaryInt, r.streamInt), srv)
}

// InterceptServer returns a new service description that will intercepts RPCs
// with the given interceptors. If both given interceptors are nil, returns
// svcDesc.
func InterceptServer(svcDesc *grpc.ServiceDesc, unaryInt grpc.UnaryServerInterceptor, streamInt grpc.StreamServerInterceptor) *grpc.ServiceDesc {
	if unaryInt == nil && streamInt == nil {
		return svcDesc
	}
	intercepted := *svcDesc

	if unaryInt != nil {
		intercepted.Methods = make([]grpc.MethodDesc, len(svcDesc.Methods))
		for i, md := range svcDesc.Methods {
			origHandler := md.Handler
			intercepted.Methods[i] = grpc.MethodDesc{
				MethodName: md.MethodName,
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					combinedInterceptor := unaryInt
					if interceptor != nil {
						// combine unaryInt with the interceptor provided to handler
						combinedInterceptor = func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
							h := func(ctx context.Context, req any) (any, error) {
								return unaryInt(ctx, req, info, handler)
							}
							// we first call provided interceptor, but supply a handler that will call unaryInt
							return interceptor(ctx, req, info, h)
						}
					}
					return origHandler(srv, ctx, dec, combinedInterceptor)
				},
			}
		}
	}

	if streamInt != nil {
		intercepted.Streams = make([]grpc.StreamDesc, len(svcDesc.Streams))
		for i, sd := range svcDesc.Streams {
			origHandler := sd.Handler
			info := &grpc.StreamServerInfo{
				FullMethod:     fmt.Sprintf("/%s/%s", svcDesc.ServiceName, sd.StreamName),
				IsClientStream: sd.ClientStreams,
				IsServerStream: sd.ServerStreams,
			}
			intercepted.Streams[i] = grpc.StreamDesc{
				StreamName:    sd.StreamName,
				ClientStreams: sd.ClientStreams,
				ServerStreams: sd.ServerStreams,
				Handler: func(srv any, stream grpc.ServerStream) error {
					return streamInt(srv, stream, info, origHandler)
				},
			}
		}
	}

	return &intercepted
}

func chainUnaryServer(unaryInt []grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	if len(unaryInt) == 1 {
		return unaryInt[0]
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		for i := range unaryInt {
			currInterceptor := unaryInt[len(unaryInt)-i-1] // going backwards through the chain
			currHandler := handler
			handler = func(ctx context.Context, req any) (any, error) {
				return currInterceptor(ctx, req, info, currHandler)
			}
		}
		return unaryInt[0](ctx, req, info, handler)
	}
}

func chainStreamServer(streamInt []grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	if len(streamInt) == 1 {
		return streamInt[0]
	}
	return func(impl any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		for i := range streamInt {
			currInterceptor := streamInt[len(streamInt)-i-1] // going backwards through the chain
			currHandler := handler
			handler = func(impl any, stream grpc.ServerStream) error {
				return currInterceptor(impl, stream, info, currHandler)
			}
		}
		return streamInt[0](impl, stream, info, handler)
	}
}
