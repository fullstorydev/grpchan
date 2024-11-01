package internal

import (
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
)

func GetCodec(name string) encoding.Codec {
	result := encoding.GetCodec(name)
	if result != nil {
		return result
	}
	resultv2 := encoding.GetCodecV2(name)
	if resultv2 == nil {
		return nil
	}
	return codecV2Adapter{resultv2}
}

type codecV2Adapter struct {
	v2 encoding.CodecV2
}

func (c codecV2Adapter) Marshal(v any) ([]byte, error) {
	buffers, err := c.v2.Marshal(v)
	if err != nil {
		return nil, err
	}
	return buffers.Materialize(), nil
}

func (c codecV2Adapter) Unmarshal(data []byte, v any) error {
	return c.v2.Unmarshal(mem.BufferSlice{mem.SliceBuffer(data)}, v)
}

func (c codecV2Adapter) Name() string {
	return c.v2.Name()
}
