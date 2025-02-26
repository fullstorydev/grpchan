package httpgrpc

import (
	"mime"

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
	// Only unary calls are supported; streams with JSON encoding are not supported.
	// Use `encoding.RegisterCodecV2` to override the default encoder with a custom encoder.
	ApplicationJson = "application/json"
)

func getUnaryCodec(contentType string) encoding.CodecV2 {
	// Ignore any errors or charsets for now, just parse the main type.
	// TODO: should this be more picky / return an error?  Maybe charset utf8 only?
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if mediaType == UnaryRpcContentType_V1 {
		return encoding.GetCodecV2(grpcproto.Name)
	}

	if mediaType == ApplicationJson {
		return encoding.GetCodecV2("json")
	}

	return nil
}

func getStreamingCodec(contentType string) encoding.CodecV2 {
	// Ignore any errors or charsets for now, just parse the main type.
	// TODO: should this be more picky / return an error?  Maybe charset utf8 only?
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if mediaType == StreamRpcContentType_V1 {
		return encoding.GetCodecV2(grpcproto.Name)
	}

	if mediaType == ApplicationJson {
		// TODO: support half-duplix JSON streaming?
		// https://en.wikipedia.org/wiki/JSON_streaming#Record_separator-delimited_JSON
		return nil
	}

	return nil
}
