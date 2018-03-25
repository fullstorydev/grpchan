// Package httpgrpc contains code for using HTTP 1.1 for GRPC calls. This is intended only
// for environments where real GRPC is not possible or prohibitively expensive, like Google
// App Engine. It could possibly be used to perform GRPC operations from a browser, however
// no client implementation, other than a Go client, is provided.
//
// For servers, RPC handlers will be invoked directly from an HTTP request, optionally
// transiting through a server interceptor. Importantly, this does not transform the
// request and then proxy it on loopback to the actual GRPC server. So GRPC service
// handlers are dispatched directly from HTTP server handlers.
//
// Caveats
//
// There are couple of limitations when using this package:
//   1. True bidi streams are not supported. The best that can be done are half-duplex
//      bidi streams, where the client uploads its entire streaming request and then the
//      server can reply with a streaming response. Interleaved reading and writing does
//      not work with HTTP 1.1. (Even if there were clients that supported it, the Go HTTP
//      server APIS do not -- once a server handler starts writing to the response body,
//      the request body is closed and no more messages can be read from it).
//   2. Client-side interceptors that interact with the *grpc.ClientConn, such as examining
//      connection states or querying static method configs, will not work. No GRPC
//      client connection is actually established and HTTP 1.1 calls will supply a nil
//      *grpc.ClientConn to any interceptor.
//
// Note that for environments like Google App Engine, which do not support streaming, use
// of streaming RPCs may result in high latency and high memory usage as entire streams must
// be buffered in memory. Use streams judiciously when inter-operating with App Engine.
//
// This package does not attempt to block use of full-duplex streaming. So if HTTP 1.1 is
// used to invoke a bidi streaming method, the RPC will almost certainly fail because the
// server's sending of headers and the first response message will immediately close the
// request side for reading. So later attempts to read a request message will fail.
//
// Anatomy of GRPC-over-HTTP
//
// A unary RPC is the simplest: the request will be a POST message and the request path
// will be the base URL's path (if any) plus "/service.name/method" (where service.name and
// method represent the fully-qualified proto service name and the method name for the
// unary method being invoked). Request metadata are used directly as HTTP request headers.
// The request payload is the binary-encoded form of the request proto, and the content-type
// is "application/x-protobuf". The response includes the best match for an HTTP status code
// based on the GRPC status code. But the response also includes a special response header,
// "X-GRPC-Status", that encodes the actual GRPC status code and message in a "code:message"
// string format. The response body is the binary-encoded form of the response proto, but
// will be empty when the GRPC status code is not "OK". If the RPC failed and the error
// includes details, they are attached via one or more headers named "X-GRPC-Details". If
// more than one error detail is associated with the status, there will be more than one
// header, and they will be added to the response in the same order as they appear in the
// server-side status. The value for the details header is a base64-encoding
// google.protobuf.Any message, which contains the error detail message. If the handler
// sends trailers, not just headers, they are encoded as HTTP 1.1 headers, but their names
// are prefixed with "X-GRPC-Trailer-". This allows clients to recover headers and trailers
// independently, as the server handler intended them.
//
// Streaming RPCs are a bit more complex. Since the payloads can include multiple messages,
// the content type is not "application/x-protobuf". It is instead "application/x-httpgrpc-proto+v1".
// The actual request and response bodies consist of a sequence of length-delimited proto
// messages, each of which is binary encoded. The length delimiter is a 32-bit prefix that
// indicates the size of the subsequent message. Response sequences have a special final
// message that is encoded with a negative size (e.g. if the message size were 15, it would
// be written as -15 on the wire in the 32-bit prefix). The type of this special final
// message is always HttpTrailer, whereas the types of all other messages in the sequence
// are that of the method's request proto. The HttpTrailer final message indicates the final
// disposition of the stream (e.g. a GRPC status code and error details) as well as any
// trailing metadata. Because the status code is not encoded until the end of the response
// payload, the HTTP status code (which is the first line of the reply) will be 200 OK.
//
// For clients that support streaming, client and server streams both work over HTTP 1.1.
// However, bidirectional streaming methods can only work if they are "half-duplex", where
// the client fully sends all request messages and then the server fully sends all response
// messages (e.g. the invocation timeline can have no interleaving/overlapping of request
// and response messages).
package httpgrpc

//go:generate protoc --go_out=. httpgrpc.proto
