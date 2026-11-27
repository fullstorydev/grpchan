package httpgrpc

import (
	"fmt"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

const jsonCodecName = "json"

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

func (c jsonCodec) Marshal(v any) (mem.BufferSlice, error) {
	msg, err := asProtoMessage(v)
	if err != nil {
		return nil, err
	}
	bb, err := grpcJsonMarshaler.Marshal(msg)
	return mem.BufferSlice{mem.SliceBuffer(bb)}, err
}

func (c jsonCodec) Unmarshal(data mem.BufferSlice, v any) error {
	msg, err := asProtoMessage(v)
	if err != nil {
		return err
	}
	return grpcJsonUnmarshaler.Unmarshal(data.Materialize(), msg)
}

func (c jsonCodec) Name() string {
	return jsonCodecName
}

func asProtoMessage(v any) (proto.Message, error) {
	msg, ok := v.(proto.Message)
	if ok {
		return msg, nil
	}
	msgV1, ok := v.(protoadapt.MessageV1)
	if ok {
		return protoadapt.MessageV2Of(msgV1), nil
	}
	return nil, fmt.Errorf("%T does not implement proto.Message", v)
}
