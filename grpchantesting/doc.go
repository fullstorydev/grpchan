// Package grpchantesting helps with testing implementations of alternate gRPC
// transports. Its main value is in a method that, given a channel, will ensure
// the channel behaves correctly under various conditions.
//
// It tests successful RPCs, failures, timeouts and client-side cancellations.
// It also covers all kinds of RPCs: unary, client-streaming, server-streaming
// and bidirectional-streaming. It can optionally test full-duplex bidi streams
// if the underlying channel supports that.
//
// The channel must be connected to a server that exposes the test server
// implementation contained in this package: &grpchantesting.TestServer{}
package grpchantesting
