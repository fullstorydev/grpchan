package main

import (
	"testing"

	"github.com/jhump/protoreflect/grpcreflect"
)

// we use bash to mv the file after it is generated so that it will end in "_test.go"
// (so it's just a test file and not linked into the actual protoc-gen-grpchan command)
//go:generate bash -c "protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative gen_test.proto && mv ./gen_test.pb.go ./gen_test_pb_test.go && mv ./gen_test_grpc.pb.go ./gen_test_grpc_pb_test.go"

func TestStreamOrder(t *testing.T) {
	// we get the service descriptor (same descriptor that protoc-gen-grpchan processes
	sd, err := grpcreflect.LoadServiceDescriptor(&TestStreams_ServiceDesc)
	if err != nil {
		t.Fatalf("failed to load service descriptor: %v", err)
	}

	// loop through stream methods just as protoc-gen-grpchan does to emit code
	streamCount := 0
	for _, md := range sd.GetMethods() {
		if md.IsClientStreaming() || md.IsServerStreaming() {
			// verify that the stream at current index is correct
			// (code emits this index when querying the serviceDesc, so we must
			// be certain that this index is right!)
			strDesc := TestStreams_ServiceDesc.Streams[streamCount]
			if md.GetName() != strDesc.StreamName {
				t.Fatalf("wrong stream at %d: %s != %s", streamCount, md.GetName(), strDesc.StreamName)
			}
			if md.IsClientStreaming() != strDesc.ClientStreams || md.IsServerStreaming() != strDesc.ServerStreams {
				t.Fatalf("wrong stream type at %d", streamCount)
			}

			streamCount++
		}
	}

	// sanity check that we saw all of the streams
	if streamCount != 7 {
		t.Fatalf("processed wrong number of methods: %d != %d", streamCount, 7)
	}
}
