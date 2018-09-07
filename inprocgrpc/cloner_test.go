package inprocgrpc

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/fullstorydev/grpchan/httpgrpc"
)

var source proto.Message

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
}

func TestDefaultCloner(t *testing.T) {
	testCloner(t, DefaultCloner{})
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
	testCloner(t, CloneFunc(func(in interface{}) interface{} {
		return proto.Clone(in.(proto.Message))
	}))
}

func TestCopyFunc(t *testing.T) {
	testCloner(t, CopyFunc(func(in, out interface{}) error {
		if reflect.TypeOf(in) != reflect.TypeOf(out) {
			return fmt.Errorf("type mismatch: %T != %T", in, out)
		}
		if reflect.ValueOf(out).IsNil() {
			return fmt.Errorf("out must not be nil")
		}
		inM := in.(proto.Message)
		outM := out.(proto.Message)
		outM.Reset()
		proto.Merge(outM, inM)
		return nil
	}))
}

func testCloner(t *testing.T, cloner Cloner) {
	dest := &httpgrpc.HttpTrailer{}
	err := cloner.Copy(source, dest)
	if err != nil {
		t.Fatalf("Copy returned unexpected error: %v", err)
	}
	if !proto.Equal(source, dest) {
		t.Fatalf("Copy failed to produce a value equal to input")
	}

	clone := cloner.Clone(source)
	if !proto.Equal(source, clone.(proto.Message)) {
		t.Fatalf("Clone failed to produce a value equal to input")
	}
}
