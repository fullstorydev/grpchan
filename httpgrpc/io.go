package httpgrpc

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/fullstorydev/grpchan/internal/sse"
	grpcproto "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
)

const (
	maxMessageSize = 100 * 1024 * 1024 // 100mb
)

// writeSizePreface writes the given 32-bit size to the given writer.
func writeSizePreface(w io.Writer, sz int32) error {
	return binary.Write(w, binary.BigEndian, sz)
}

// writeProtoMessage writes a length-delimited proto message to the given
// writer. This writes the size preface, indicating the size of the encoded
// message, followed by the actual message contents. If end is true, the
// size is written as a negative value, indicating to the receiver that this
// is the last message in the stream. (The last message should be an instance
// of HttpTrailer.)
func writeProtoMessage(w io.Writer, codec encoding.CodecV2, m interface{}, end bool) error {
	buf, err := codec.Marshal(m)
	if err != nil {
		return err
	}
	b := buf.Materialize()

	sz := len(b)
	if sz > math.MaxInt32 {
		return fmt.Errorf("message too large to send: %d bytes", sz)
	}
	if end {
		// trailer message is indicated w/ negative size
		sz = -sz
	}
	err = writeSizePreface(w, int32(sz))
	if err != nil {
		return err
	}

	_, err = w.Write(b)
	if err == nil {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	return err
}

// readSizePreface reads a 32-bit size from the given reader. If the value is
// negative, it indicates the last message in the stream. Messages can have zero
// size, but the last message in the stream should never have zero size (so its
// size will be negative).
func readSizePreface(in io.Reader) (int32, error) {
	var sz int32
	err := binary.Read(in, binary.BigEndian, &sz)
	return sz, err
}

// asMetadata converts the given HTTP headers into GRPC metadata.
func asMetadata(header http.Header) (metadata.MD, error) {
	// metadata has same shape as http.Header,
	md := metadata.MD{}
	for k, vs := range header {
		k = strings.ToLower(k)
		for _, v := range vs {
			if strings.HasSuffix(k, "-bin") {
				vv, err := base64.URLEncoding.DecodeString(v)
				if err != nil {
					return nil, err
				}
				v = string(vv)
			}
			md[k] = append(md[k], v)
		}
	}
	return md, nil
}

var reservedHeaders = map[string]struct{}{
	"accept-encoding":   {},
	"connection":        {},
	"content-type":      {},
	"content-length":    {},
	"keep-alive":        {},
	"te":                {},
	"trailer":           {},
	"transfer-encoding": {},
	"upgrade":           {},
}

func toHeaders(md metadata.MD, h http.Header, prefix string) {
	// binary headers must be base-64-encoded
	for k, vs := range md {
		lowerK := strings.ToLower(k)
		if _, ok := reservedHeaders[lowerK]; ok {
			// ignore reserved header keys
			continue
		}
		isBin := strings.HasSuffix(lowerK, "-bin")
		for _, v := range vs {
			if isBin {
				v = base64.URLEncoding.EncodeToString([]byte(v))
			}
			h.Add(prefix+k, v)
		}
	}
}

type strAddr string

func (a strAddr) Network() string {
	if a != "" {
		// Per the documentation on net/http.Request.RemoteAddr, if this is
		// set, it's set to the IP:port of the peer (hence, TCP):
		// https://golang.org/pkg/net/http/#Request
		//
		// If we want to support Unix sockets later, we can
		// add our own grpc-specific convention within the
		// grpc codebase to set RemoteAddr to a different
		// format, or probably better: we can attach it to the
		// context and use that from serverHandlerTransport.RemoteAddr.
		return "tcp"
	}
	return ""
}

func (a strAddr) String() string { return string(a) }

type streamReader func() (streamMsg, error)

type streamWriter func(m any, isTrailer bool) error

type flusher interface {
	Flush() error
}

func getServerStreamWriter(mediaType string, w io.Writer, flusher flusher) (streamWriter, string) {
	if mediaType == StreamRpcContentType_V1 {
		codec := encoding.GetCodecV2(grpcproto.Name)
		return newSizePrefixedWriter(w, codec), StreamRpcContentType_V1
	}

	if mediaType == ApplicationJson {
		codec := encoding.GetCodecV2("json")
		return newSSEWriter(w, flusher, codec), EventStreamContentType
	}

	return nil, ""
}

func getServerStreamReader(mediaType string, r io.Reader) streamReader {
	if mediaType == StreamRpcContentType_V1 {
		codec := encoding.GetCodecV2(grpcproto.Name)
		return newSizePrefixedReader(r, codec)
	}

	if mediaType == ApplicationJson {
		codec := encoding.GetCodecV2("json")
		return newJSONReader(r, codec)
	}

	return nil
}

func getClientStreamReader(mediaType string, r io.Reader) streamReader {
	if mediaType == StreamRpcContentType_V1 {
		codec := encoding.GetCodecV2(grpcproto.Name)
		return newSizePrefixedReader(r, codec)
	}

	if mediaType == EventStreamContentType {
		codec := encoding.GetCodecV2("json")
		return newSSEReader(r, codec)
	}

	return nil
}

func getClientStreamWriter(mediaType string, w io.Writer) streamWriter {
	if mediaType == StreamRpcContentType_V1 {
		codec := encoding.GetCodecV2(grpcproto.Name)
		return newSizePrefixedWriter(w, codec)
	}

	if mediaType == ApplicationJson {
		codec := encoding.GetCodecV2("json")
		return newJSONWriter(w, codec)
	}

	return nil
}

type streamMsg struct {
	codec     encoding.CodecV2
	data      []byte
	isTrailer bool
}

func (s *streamMsg) Decode(m any) error {
	return s.codec.Unmarshal(mem.BufferSlice{mem.SliceBuffer(s.data)}, m)
}

func newSizePrefixedReader(r io.Reader, codec encoding.CodecV2) func() (streamMsg, error) {
	return func() (streamMsg, error) {
		size, err := readSizePreface(r)
		if err != nil {
			return streamMsg{}, err
		}

		isTrailer := size < 0
		if isTrailer {
			size = -size
		}

		if size > maxMessageSize {
			return streamMsg{}, fmt.Errorf("bad size preface: indicated size is too large: %d", size)
		}

		data := make([]byte, size)
		_, err = io.ReadAtLeast(r, data, int(size))
		if err != nil {
			return streamMsg{}, err
		}

		return streamMsg{
			codec:     codec,
			data:      data,
			isTrailer: isTrailer,
		}, nil
	}
}

func newSizePrefixedWriter(w io.Writer, codec encoding.CodecV2) func(m any, isTrailer bool) error {
	return func(m any, isTrailer bool) error {
		return writeProtoMessage(w, codec, m, isTrailer)
	}
}

func newJSONReader(r io.Reader, codec encoding.CodecV2) func() (streamMsg, error) {
	d := json.NewDecoder(r)
	return func() (streamMsg, error) {
		var msg json.RawMessage
		if err := d.Decode(&msg); err != nil {
			return streamMsg{}, err
		}

		return streamMsg{
			codec: codec,
			data:  msg,
		}, nil
	}
}

func newJSONWriter(w io.Writer, codec encoding.CodecV2) func(m any, isTrailer bool) error {
	return func(m any, isTrailer bool) error {
		if isTrailer {
			panic("trailers are not supported for JSON")
		}

		data, err := codec.Marshal(m)
		if err != nil {
			return err
		}

		_, err = w.Write(data.Materialize())
		return err
	}
}

func newSSEWriter(w io.Writer, flusher flusher, codec encoding.CodecV2) func(m any, isTrailer bool) error {
	e := sse.NewEncoder(w)
	return func(m any, isTrailer bool) error {
		data, err := codec.Marshal(m)
		if err != nil {
			return err
		}

		if isTrailer {
			if err := e.Encode(&sse.Event{
				Type: "trailer",
				Data: data.Materialize(),
			}); err != nil {
				return err
			}
		} else {
			if err := e.Encode(&sse.Event{
				Data: data.Materialize(),
			}); err != nil {
				return err
			}
		}

		return flusher.Flush()
	}
}

func newSSEReader(r io.Reader, codec encoding.CodecV2) func() (streamMsg, error) {
	d := sse.NewDecoder(r)

	return func() (streamMsg, error) {
		event, err := d.Decode()
		if err != nil {
			return streamMsg{}, err
		}

		return streamMsg{
			codec:     codec,
			data:      event.Data,
			isTrailer: event.Type == "trailer",
		}, nil
	}
}
