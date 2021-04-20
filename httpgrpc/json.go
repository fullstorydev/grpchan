package httpgrpc

import (
	"bytes"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/encoding"
)

var (
	grpcJsonMarshaler = jsonpb.Marshaler{
		EnumsAsInts:  true,
		EmitDefaults: true,
	}

	grpcJsonUnmarshaler = jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}
)

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (c jsonCodec) Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := grpcJsonMarshaler.Marshal(&buf, v.(proto.Message))
	return buf.Bytes(), err
}

func (c jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return grpcJsonUnmarshaler.Unmarshal(bytes.NewReader(data), v.(proto.Message))

}

func (c jsonCodec) Name() string {
	return "json"
}
