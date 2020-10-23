package httpgrpc

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/internal"
)

// Mux is a function that can register a gRPC-over-HTTP handler. This is used to
// register handlers in bulk for an RPC service. Its signature matches that of
// the HandleFunc method of the http.ServeMux type, and it also matches that of
// the http.HandleFunc function (for registering handlers with the default mux).
//
// Callers can provide custom Mux functions that further decorate the handler
// (for example, adding authentication checks, logging, error handling, etc).
type Mux func(pattern string, handler func(http.ResponseWriter, *http.Request))

// HandlerOption is an option to customize some aspect of the HTTP handler
// behavior, such as rendering gRPC errors to HTTP responses.
type HandlerOption func(*handlerOpts)

type handlerOpts struct {
	errFunc func(context.Context, *status.Status, http.Header) (httpCode int)
}

// ErrorRenderer returns a HandlerOption that will cause the handler to use the
// given function to render an error.  It is only used for unary RPCs since
// streaming RPCs serialize a status message to the response trailer (in the
// HTTP body) instead.
//
// The function can examine and add response header values via the supplied
// http.Header. It can also examine the request context. It must then return
// the HTTP status code to use.
//
// If no such option is used the handler will use DefaultErrorRenderer.
func ErrorRenderer(errFunc func(reqCtx context.Context, st *status.Status, rspHdr http.Header) (httpCode int)) HandlerOption {
	return func(h *handlerOpts) {
		h.errFunc = errFunc
	}
}

// DefaultErrorRenderer translates the gRPC code in the given status to an HTTP
// status code. The following table shows how status codes are translated:
//   Canceled:         * 502 Bad Gateway
//   Unknown:            500 Internal Server Error
//   InvalidArgument:    400 Bad Request
//   DeadlineExceeded: * 504 Gateway Timeout
//   NotFound:           404 Not Found
//   AlreadyExists:      409 Conflict
//   PermissionDenied:   403 Forbidden
//   Unauthenticated:    401 Unauthorized
//   ResourceExhausted:  429 Too Many Requests
//   FailedPrecondition: 412 Precondition Failed
//   Aborted:            409 Conflict
//   OutOfRange:         422 Unprocessable Entity
//   Unimplemented:      501 Not Implemented
//   Internal:           500 Internal Server Error
//   Unavailable:        503 Service Unavailable
//   DataLoss:           500 Internal Server Error
//
//   * If the gRPC status indicates Canceled or DeadlineExceeded
//     and the given request context ALSO indicates a context error
//     (meaning that the request was cancelled by the client), then
//     a 499 Client Closed Request code is used instead.
//
// If any other gRPC status code is observed, it would get translated into a
// 500 Internal Server Error.
//
// Note that OK is absent from the mapping because the error renderer will never
// be called for a non-error status.
func DefaultErrorRenderer(ctx context.Context, st *status.Status, _ http.Header) (httpCode int) {
	if (st.Code() == codes.Canceled || st.Code() == codes.DeadlineExceeded) && ctx.Err() != nil {
		return 499
	}
	return httpStatusFromCode(st.Code())
}

// HandleServices uses the given mux to register handlers for all methods
// exposed by handlers registered in reg. They are registered using a path of
// "basePath/name.of.Service/Method". If non-nil interceptor(s) are provided
// then they will be used to intercept applicable RPCs before dispatch to the
// registered handler.
func HandleServices(mux Mux, basePath string, reg grpchan.HandlerMap, unaryInt grpc.UnaryServerInterceptor, streamInt grpc.StreamServerInterceptor, opts ...HandlerOption) {
	var hOpts handlerOpts
	for _, opt := range opts {
		opt(&hOpts)
	}

	reg.ForEach(func(desc *grpc.ServiceDesc, svr interface{}) {
		for i := range desc.Methods {
			md := desc.Methods[i]
			h := handleMethod(svr, desc.ServiceName, &md, unaryInt, &hOpts)
			mux(path.Join(basePath, fmt.Sprintf("%s/%s", desc.ServiceName, md.MethodName)), h)
		}
		for i := range desc.Streams {
			sd := desc.Streams[i]
			h := handleStream(svr, desc.ServiceName, &sd, streamInt, &hOpts)
			mux(path.Join(basePath, fmt.Sprintf("%s/%s", desc.ServiceName, sd.StreamName)), h)
		}
	})
}

// HandleMethod returns an HTTP handler that will handle a unary RPC method
// by dispatching the given method on the given server.
func HandleMethod(svr interface{}, serviceName string, desc *grpc.MethodDesc, unaryInt grpc.UnaryServerInterceptor, opts ...HandlerOption) http.HandlerFunc {
	var hOpts handlerOpts
	for _, opt := range opts {
		opt(&hOpts)
	}
	return handleMethod(svr, serviceName, desc, unaryInt, &hOpts)
}

func handleMethod(svr interface{}, serviceName string, desc *grpc.MethodDesc, unaryInt grpc.UnaryServerInterceptor, opts *handlerOpts) http.HandlerFunc {
	errHandler := opts.errFunc
	if errHandler == nil {
		errHandler = DefaultErrorRenderer
	}
	fullMethod := fmt.Sprintf("/%s/%s", serviceName, desc.MethodName)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if p := peerFromRequest(r); p != nil {
			ctx = peer.NewContext(ctx, p)
		}
		defer drainAndClose(r.Body)
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed)
			return
		}

		// NB: This is where support for a second of the protocol would be implemented. This
		// check would instead need to also accept a second content-type and the logic below
		// for consuming the request and sending the response would need to switch based on
		// the actual version in use.
		if r.Header.Get("Content-Type") != UnaryRpcContentType_V1 {
			writeError(w, http.StatusUnsupportedMediaType)
			return
		}

		ctx, err := contextFromHeaders(ctx, r.Header)
		if err != nil {
			writeError(w, http.StatusBadRequest)
			return
		}

		req, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, 499)
			return
		}

		dec := func(msg interface{}) error {
			return proto.Unmarshal(req, msg.(proto.Message))
		}
		sts := internal.UnaryServerTransportStream{Name: fullMethod}
		resp, err := desc.Handler(svr, grpc.NewContextWithServerTransportStream(ctx, &sts), dec, unaryInt)
		toHeaders(sts.GetHeaders(), w.Header(), "")
		toHeaders(sts.GetTrailers(), w.Header(), "X-GRPC-Trailer-")
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.OK {
				// preserve all error details, but rewrite the code since we don't want
				// to send back a non-error status when we know an error occured
				stpb := st.Proto()
				stpb.Code = int32(codes.Internal)
				st = status.FromProto(stpb)
			}
			statProto := st.Proto()
			w.Header().Set("X-GRPC-Status", fmt.Sprintf("%d:%s", statProto.Code, statProto.Message))
			for _, d := range statProto.Details {
				b, err := proto.Marshal(d)
				if err != nil {
					continue
				}
				str := base64.RawURLEncoding.EncodeToString(b)
				w.Header().Add(grpcDetailsHeader, str)
			}
			httpStatus := errHandler(r.Context(), st, w.Header())
			writeError(w, httpStatus)
			return
		}

		b, err := proto.Marshal(resp.(proto.Message))
		if err != nil {
			writeError(w, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", UnaryRpcContentType_V1)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
		w.Write(b)
	}
}

// HandleStream returns an HTTP handler that will handle a streaming RPC method
// by dispatching the given method on the given server.
func HandleStream(svr interface{}, serviceName string, desc *grpc.StreamDesc, streamInt grpc.StreamServerInterceptor, opts ...HandlerOption) http.HandlerFunc {
	var hOpts handlerOpts
	for _, opt := range opts {
		opt(&hOpts)
	}
	return handleStream(svr, serviceName, desc, streamInt, &hOpts)
}

func handleStream(svr interface{}, serviceName string, desc *grpc.StreamDesc, streamInt grpc.StreamServerInterceptor, opts *handlerOpts) http.HandlerFunc {
	info := &grpc.StreamServerInfo{
		FullMethod:     fmt.Sprintf("/%s/%s", serviceName, desc.StreamName),
		IsClientStream: desc.ClientStreams,
		IsServerStream: desc.ServerStreams,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if p := peerFromRequest(r); p != nil {
			ctx = peer.NewContext(ctx, p)
		}
		defer drainAndClose(r.Body)
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed)
			return
		}

		// NB: This is where support for a second of the protocol would be implemented. This
		// check would instead need to also accept a second content-type and the logic below
		// for consuming the request and sending the response would need to switch based on
		// the actual version in use.
		if r.Header.Get("Content-Type") != StreamRpcContentType_V1 {
			writeError(w, http.StatusUnsupportedMediaType)
			return
		}

		ctx, err := contextFromHeaders(ctx, r.Header)
		if err != nil {
			writeError(w, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", StreamRpcContentType_V1)

		str := &serverStream{r: r, w: w, respStream: desc.ClientStreams}
		sts := internal.ServerTransportStream{Name: info.FullMethod, Stream: str}
		str.ctx = grpc.NewContextWithServerTransportStream(ctx, &sts)
		if streamInt != nil {
			err = streamInt(svr, str, info, desc.Handler)
		} else {
			err = desc.Handler(svr, str)
		}
		if str.writeFailed {
			// nothing else we can do
			return
		}

		tr := HttpTrailer{
			Code:     int32(codes.OK),
			Message:  codes.OK.String(),
			Metadata: asTrailerProto(metadata.Join(str.tr...)),
		}
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.OK {
				// preserve all error details, but rewrite the code since we don't want
				// to send back a non-error status when we know an error occured
				stpb := st.Proto()
				stpb.Code = int32(codes.Internal)
				st = status.FromProto(stpb)
			}
			statProto := st.Proto()
			tr.Code = statProto.Code
			tr.Message = statProto.Message
			tr.Details = statProto.Details
		}

		writeProtoMessage(w, &tr, true)
	}
}

func peerFromRequest(r *http.Request) *peer.Peer {
	pr := peer.Peer{Addr: strAddr(r.RemoteAddr)}
	if r.TLS != nil {
		pr.AuthInfo = credentials.TLSInfo{State: *r.TLS}
	}
	return &pr
}

func drainAndClose(r io.ReadCloser) error {
	_, copyErr := io.Copy(ioutil.Discard, r)
	closeErr := r.Close()
	// error from io.Copy likely more useful than the one from Close
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func writeError(w http.ResponseWriter, code int) {
	msg := http.StatusText(code)
	if msg == "" {
		if code == 499 {
			msg = "Client Closed Request"
		} else {
			msg = "Unknown"
		}
	}
	http.Error(w, msg, code)
}

// asTrailerProto converts the given metadata into a map that can be used with
// HttpTrailer to convey trailers back to the caller via a final message in the
// response body.
func asTrailerProto(md metadata.MD) map[string]*TrailerValues {
	result := map[string]*TrailerValues{}
	for k, vs := range md {
		tvs := TrailerValues{}
		tvs.Values = append(tvs.Values, vs...)
		result[k] = &tvs
	}
	return result
}

// serverStream implements a server stream over HTTP 1.1.
type serverStream struct {
	ctx context.Context
	// respStream is set to indicate whether client expects stream response; unary if false
	respStream bool

	// rmu serializes access to r and protects recvd
	rmu sync.Mutex
	r   *http.Request
	// recvd tracks the number of request messages received
	recvd int

	// wmu serializes access to w and protects headersSent, writeFailed, and tr
	wmu         sync.Mutex
	w           http.ResponseWriter
	headersSent bool
	writeFailed bool
	tr          []metadata.MD
}

func (s *serverStream) SetHeader(md metadata.MD) error {
	return s.setHeader(md, false)
}

func (s *serverStream) SendHeader(md metadata.MD) error {
	return s.setHeader(md, true)
}

func (s *serverStream) setHeader(md metadata.MD, send bool) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()

	if s.headersSent {
		return errors.New("headers already sent")
	}

	h := s.w.Header()
	toHeaders(md, h, "")

	if send {
		s.w.WriteHeader(http.StatusOK)
		s.headersSent = true
	}

	return nil
}

func (s *serverStream) SetTrailer(md metadata.MD) {
	s.wmu.Lock()
	defer s.wmu.Unlock()

	s.tr = append(s.tr, md)
}

func (s *serverStream) Context() context.Context {
	return s.ctx
}

func (s *serverStream) SendMsg(m interface{}) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()

	if s.writeFailed {
		// strange, but simulates what happens in real GRPC: stream
		// is closed after a write failure, and trying to send message
		// on a closed stream returns EOF
		return io.EOF
	}

	s.headersSent = true // sent implicitly
	err := writeProtoMessage(s.w, m.(proto.Message), false)
	if err != nil {
		s.writeFailed = true
	}
	return err
}

func (s *serverStream) RecvMsg(m interface{}) error {
	s.rmu.Lock()
	defer s.rmu.Unlock()

	if !s.respStream && s.recvd > 0 {
		return io.EOF
	}

	s.recvd++

	size, err := readSizePreface(s.r.Body)
	if err != nil {
		return err
	}

	err = readProtoMessage(s.r.Body, size, m.(proto.Message))
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	} else if err != nil {
		return err
	}

	if !s.respStream {
		_, err = readSizePreface(s.r.Body)
		if err != io.EOF {
			// client tried to send >1 message!
			return status.Error(codes.InvalidArgument, "method accepts 1 request message but client sent >1")
		}
	}

	return nil
}

// contextFromHeaders returns a child of the given context that is populated
// using the given headers. The headers are converted to incoming metadata that
// can be retrieved via metadata.FromIncomingContext. If the headers contain a
// GRPC timeout, that is used to create a timeout for the returned context.
func contextFromHeaders(parent context.Context, h http.Header) (context.Context, error) {
	md, err := asMetadata(h)
	if err != nil {
		return nil, err
	}
	ctx := metadata.NewIncomingContext(parent, md)

	// deadline propagation
	timeout := h.Get("GRPC-Timeout")
	if timeout != "" {
		// See GRPC wire format, "Timeout" component of request: https://grpc.io/docs/guides/wire.html#requests
		suffix := timeout[len(timeout)-1]
		if timeoutVal, err := strconv.ParseInt(timeout[:len(timeout)-1], 10, 64); err == nil {
			var unit time.Duration
			switch suffix {
			case 'H':
				unit = time.Hour
			case 'M':
				unit = time.Minute
			case 'S':
				unit = time.Second
			case 'm':
				unit = time.Millisecond
			case 'u':
				unit = time.Microsecond
			case 'n':
				unit = time.Nanosecond
			}
			if unit != 0 {
				ctx, _ = context.WithTimeout(ctx, time.Duration(timeoutVal)*unit)
			}
		}
	}
	return ctx, nil
}
