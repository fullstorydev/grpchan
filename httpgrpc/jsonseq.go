package httpgrpc

import (
	"bytes"
	"google.golang.org/grpc/encoding"
)

func init() {
	encoding.RegisterCodec(jsonSeqCodec{})
}

type jsonSeqCodec struct{}

// Marshal uses the registered json codec to marshal data into a json sequence compatible with RFC7464, prefixing with
// an ASCII record separator (0x1E) and suffixing with an ASCII line feed (0x0A).
// https://www.rfc-editor.org/rfc/rfc7464.html
func (c jsonSeqCodec) Marshal(v interface{}) ([]byte, error) {
	jc := encoding.GetCodec("json")
	data, err := jc.Marshal(v)
	if err != nil {
		return nil, err
	}

	data = append([]byte{0x1E}, data...)
	data = append(data, []byte{0x0A}...)
	return data, nil
}

// Unmarshal trims any leading record separator and trailing line feed, using the registered json codec to unmarshal the
// remaining data.
func (c jsonSeqCodec) Unmarshal(data []byte, v interface{}) error {
	trimmed := bytes.TrimPrefix(data, []byte{0x1E})
	trimmed = bytes.TrimSuffix(data, []byte{0x0A})
	jc := encoding.GetCodec("json")
	return jc.Unmarshal(trimmed, v)
}

func (c jsonSeqCodec) Name() string {
	return "json-seq"
}
