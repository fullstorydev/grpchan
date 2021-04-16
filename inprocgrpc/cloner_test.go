package inprocgrpc

import (
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/fullstorydev/grpchan/httpgrpc"
)

var (
	source   *httpgrpc.HttpTrailer
	sourceJs string // snapshot of source as JSON

	jsm = &protojson.MarshalOptions{}
)

func init() {
	source = &httpgrpc.HttpTrailer{
		Code:    123,
		Message: "foobar",
		Metadata: map[string]*httpgrpc.TrailerValues{
			"abc": {Values: []string{"a", "b", "c"}},
			"def": {Values: []string{"foo", "bar", "baz"}},
			"ghi": {Values: []string{"xyz", "123"}},
		},
	}
	sourceJsBytes, err := jsm.Marshal(source)
	if err != nil {
		panic(err)
	}
	sourceJs = string(sourceJsBytes)
}

func TestProtoCloner(t *testing.T) {
	testCloner(t, ProtoCloner{})
}

type protoCodec struct{}

func (protoCodec) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

func (protoCodec) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

func (protoCodec) Name() string {
	return "proto"
}

func TestCodecCloner(t *testing.T) {
	testCloner(t, CodecCloner(protoCodec{}))
}

func TestCloneFunc(t *testing.T) {
	testCloner(t, CloneFunc(func(in interface{}) (interface{}, error) {
		return proto.Clone(in.(proto.Message)), nil
	}))
}

func TestCopyFunc(t *testing.T) {
	testCloner(t, CopyFunc(func(out, in interface{}) error {
		if reflect.TypeOf(in) != reflect.TypeOf(out) {
			return fmt.Errorf("type mismatch: %T != %T", in, out)
		}
		if reflect.ValueOf(out).IsNil() {
			return fmt.Errorf("out must not be nil")
		}
		inM := in.(proto.Message)
		outM := out.(proto.Message)
		proto.Reset(outM)
		proto.Merge(outM, inM)
		return nil
	}))
}

func testCloner(t *testing.T, cloner Cloner) {
	dest := &httpgrpc.HttpTrailer{}
	err := cloner.Copy(dest, source)
	if err != nil {
		t.Fatalf("Copy returned unexpected error: %v", err)
	}
	if !proto.Equal(source, dest) {
		t.Fatalf("Copy failed to produce a value equal to input")
	}
	checkIndependence(t, dest)

	clone, err := cloner.Clone(source)
	if err != nil {
		t.Fatalf("Clone returned unexpected error: %v", err)
	}
	if !proto.Equal(source, clone.(proto.Message)) {
		t.Fatalf("Clone failed to produce a value equal to input")
	}
	checkIndependence(t, clone.(*httpgrpc.HttpTrailer))
}

func checkIndependence(t *testing.T, dest *httpgrpc.HttpTrailer) {
	// mutate copy and make sure we don't see it in original
	// (e.g. verifies the copy is a deep copy)
	dest.Message += "baz"
	dest.Metadata["ghi"].Values = append(dest.Metadata["ghi"].Values, "456")
	dest.Metadata["jkl"] = &httpgrpc.TrailerValues{Values: []string{"zomg!"}}

	sourceJs2, err := jsm.Marshal(source)
	if err != nil {
		t.Fatalf("Failed to marsal message to JSON: %v", err)
	}
	if string(sourceJs2) != sourceJs {
		t.Errorf("source changed after mutating dest!\nExpecting:\n%s\nGot:\n%s\n", sourceJs, string(sourceJs2))
	}
}
