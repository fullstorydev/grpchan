package main

import (
	"testing"
)

// we use bash to mv the file after it is generated so that it will end in "_test.go"
// (so it's just a test file and not linked into the actual protoc-gen-grpchan command)
//go:generate bash -c "protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative gen_test.proto && mv ./gen_test.pb.go ./gen_test_pb_test.go && mv ./gen_test_grpc.pb.go ./gen_test_grpc_pb_test.go"

func TestStreamOrder(t *testing.T) {
	// we get the service descriptor (same descriptor that protoc-gen-grpchan processes
	sd := File_gen_test_proto.Services().ByName("TestStreams")
	if sd == nil {
		t.Fatalf("failed to find service descriptor")
	}

	// loop through stream methods just as protoc-gen-grpchan does to emit code
	streamCount := 0
	for i := 0; i < sd.Methods().Len(); i++ {
		md := sd.Methods().Get(i)
		if md.IsStreamingClient() || md.IsStreamingServer() {
			// verify that the stream at current index is correct
			// (code emits this index when querying the serviceDesc, so we must
			// be certain that this index is right!)
			strDesc := TestStreams_ServiceDesc.Streams[streamCount]
			if string(md.Name()) != strDesc.StreamName {
				t.Fatalf("wrong stream at %d: %s != %s", streamCount, md.Name(), strDesc.StreamName)
			}
			if md.IsStreamingClient() != strDesc.ClientStreams || md.IsStreamingServer() != strDesc.ServerStreams {
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
