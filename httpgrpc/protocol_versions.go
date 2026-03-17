package httpgrpc

import (
	"context"
	"io"
	"mime"
	"net/http"

	"google.golang.org/grpc/encoding"
	grpcproto "google.golang.org/grpc/encoding/proto"
)

// If the on-the-wire encoding every needs to be changed in a backwards-incompatible way,
// here are the steps for doing so:
//
// 1. Define new content-types that represent the new encoding. If the current encoding is
//    "v1" then increment (e.g. "v2"). (No semver here, just a single integer version...)
// 2. Update server code to switch on incoming content-type. It must continue to support
//    the previous version protocol if it sees the corresponding content-types.
//    NOTE: Servers should only support two versions at a time; let's call them Version-Now
//    and Version-Next. Version-Next should be fully deployed (in all clients and servers)
//    before any third version is conceived. That way, Version-Now support can be removed.
//    The code will be simpler if we only support to 2 versions, instead of up to N.
// 3. Create a new implementation of github.com/fullstorydev/grpchan.Channel named channelNext
//    that implements Version-Next for clients. IMPORTANT: note that the recommended name
//    is NOT exported. It should not yet be usable and should only be used for tests inside
//    this package.
// 4. Update tests so that they perform the same cases with BOTH client versions: e.g with
//    both Channel and channelNext instances. This confirms that servers correctly continue
//    to support Version-Now and also ensures that Version-Next is functional end-to-end.
// 5. Update the package documentation to describe the Version-Next protocol anatomy.
// 6. Deploy servers!!!
// 7. Only after all servers support Version-Next is it safe to export the Version-Next
//    channel implementation and use it outside of testing. At this time, you can safely
//    change channelNext to be named Channel and then remove the old code for Version-Now.

// These are the content-types used for "version 1" (hopefully the only version ever?)
// of the gRPC-over-HTTP transport
const (
	UnaryRpcContentType_V1  = "application/x-protobuf"
	StreamRpcContentType_V1 = "application/x-httpgrpc-proto+v1"
)

const (
	// Non-standard and experimental; uses the `jsonpb.Marshaler` by default.
	// Use `encoding.RegisterCodecV2` to override the default encoder with a custom encoder.
	// Unary calls use JSON encoding for the request and response. Streaming calls from the
	// client to the server use JSON encoding with mutiple values. Streaming calls from the
	// server to the client use SSE encoding with multiple events.
	ApplicationJson = "application/json"

	EventStreamContentType = "text/event-stream"
)

// getCodecForServerUnaryResponse returns the codec to use to handle a unary request on
// the server, based on the Content-Type headers from the incoming request.
func getCodecForServerUnaryResponse(contentType string) encoding.CodecV2 {
	// Ignore any errors or charsets for now, just parse the main type.
	// TODO: should this be more picky / return an error?  Maybe charset utf8 only?
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if mediaType == UnaryRpcContentType_V1 {
		return encoding.GetCodecV2(grpcproto.Name)
	}

	if mediaType == ApplicationJson {
		return encoding.GetCodecV2(jsonCodecName)
	}

	return nil
}

// getServerStreamReaderAndWriter returns the reader and writer to use to handle a streaming request on
// the server, based on the Content-Type headers from the incoming request.
func getServerStreamReaderAndWriter(contentType string, r io.Reader, w io.Writer, flusher flusher) (streamReader, streamWriter, string) {
	// Ignore any errors or charsets for now, just parse the main type.
	// TODO: should this be more picky / return an error?  Maybe charset utf8 only?
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if mediaType == StreamRpcContentType_V1 {
		codec := encoding.GetCodecV2(grpcproto.Name)
		return newSizePrefixedReader(r, codec), newSizePrefixedWriter(w, codec), StreamRpcContentType_V1
	}

	if mediaType == ApplicationJson {
		codec := encoding.GetCodecV2(jsonCodecName)
		return newJSONReader(r, codec), newSSEWriter(w, flusher, codec), EventStreamContentType
	}

	return nil, nil, ""
}

// getHeadersAndCodecForClientUnaryRequest returns the headers and codec to use to handle a unary request on
// the client, based on the the configuration of the client (currently whether to use JSON encoding).
func getHeadersAndCodecForClientUnaryRequest(ctx context.Context, useJSONEncoding bool) (http.Header, encoding.CodecV2) {
	h := headersFromContext(ctx)
	if useJSONEncoding {
		h.Set("Content-Type", ApplicationJson)
		return h, encoding.GetCodecV2(jsonCodecName)
	}

	h.Set("Content-Type", UnaryRpcContentType_V1)
	return h, encoding.GetCodecV2(grpcproto.Name)
}

// getHeadersAndCodecForClientStreamingRequest returns the headers and writer to use to handle a streaming request on
// the client, based on the the configuration of the client (currently whether to use JSON encoding).
func getHeadersAndCodecForClientStreamingRequest(ctx context.Context, useJSONEncoding bool) (http.Header, func(w io.Writer) streamWriter) {
	h := headersFromContext(ctx)
	if useJSONEncoding {
		h.Set("Content-Type", ApplicationJson)
		h.Set("Accept", EventStreamContentType)
		return h, func(w io.Writer) streamWriter {
			return newJSONWriter(w, encoding.GetCodecV2(jsonCodecName))
		}
	}

	h.Set("Content-Type", StreamRpcContentType_V1)
	h.Set("Accept", StreamRpcContentType_V1)
	return h, func(w io.Writer) streamWriter {
		return newSizePrefixedWriter(w, encoding.GetCodecV2(grpcproto.Name))
	}
}

// getClientStreamReader returns the reader to use to handle a streaming result on
// the client, based on the Content-Type header that was returned from the server.
func getClientStreamReader(contentType string, r io.Reader) streamReader {
	// Ignore any errors or charsets for now, just parse the main type.
	// TODO: should this be more picky / return an error?  Maybe charset utf8 only?
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if mediaType == StreamRpcContentType_V1 {
		codec := encoding.GetCodecV2(grpcproto.Name)
		return newSizePrefixedReader(r, codec)
	}

	if mediaType == EventStreamContentType {
		codec := encoding.GetCodecV2(jsonCodecName)
		return newSSEReader(r, codec)
	}

	return nil
}
