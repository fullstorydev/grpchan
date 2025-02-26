package httpgrpc

import (
	//lint:ignore SA1019 we use the old v1 package because
	//  we need to support older generated messages
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	grpcJsonMarshaler = protojson.MarshalOptions{
		UseEnumNumbers:  true,
		EmitUnpopulated: true,
	}

	grpcJsonUnmarshaler = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

func init() {
	encoding.RegisterCodecV2(jsonCodec{})
}

type jsonCodec struct{}

func (c jsonCodec) Marshal(v interface{}) (mem.BufferSlice, error) {
	msg := proto.MessageV2(v.(proto.Message))
	bb, err := grpcJsonMarshaler.Marshal(msg)
	return mem.BufferSlice{mem.SliceBuffer(bb)}, err
}

func (c jsonCodec) Unmarshal(data mem.BufferSlice, v interface{}) error {
	msg := proto.MessageV2(v.(proto.Message))
	return grpcJsonUnmarshaler.Unmarshal(data.Materialize(), msg)
}

func (c jsonCodec) Name() string {
	return "json"
}
