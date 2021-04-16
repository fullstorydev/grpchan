package httpgrpc

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

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
func writeProtoMessage(w io.Writer, codec encoding.Codec, m interface{}, end bool) error {
	b, err := codec.Marshal(m)
	if err != nil {
		return err
	}

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

// readProtoMessage reads data from the given reader and decodes it into the given
// message. The sz parameter indicates the  number of bytes that must be read to
// decode the proto. This does not first call readSizePreface; callers must do that
// first.
func readProtoMessage(in io.Reader, codec encoding.Codec, sz int32, m interface{}) error {
	if sz < 0 {
		return fmt.Errorf("bad size preface: size cannot be negative: %d", sz)
	} else if sz > maxMessageSize {
		return fmt.Errorf("bad size preface: indicated size is too large: %d", sz)
	}
	msg := make([]byte, sz)
	_, err := io.ReadAtLeast(in, msg, int(sz))
	if err != nil {
		return err
	}
	return codec.Unmarshal(msg, m)
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
