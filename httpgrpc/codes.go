package httpgrpc

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

// httpStatusFromCode translates the given GRPC code into an HTTP
// response. This is used to set the HTTP status code for unary RPCs.
// (Streaming RPCs cannot convey a GRPC status code until the stream
// completes, so they use a 200 HTTP status code and then encode the
// actual status, along with any trailer metadata, at the end of the
// response stream.)
func httpStatusFromCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusBadGateway
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusUnprocessableEntity
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// codeFromHttpStatus translates the given HTTP status code into a GRPC code.
// This is used for unary RPCs where the server failed to include the actual
// GRPC status code via response header.
func codeFromHttpStatus(stat int) codes.Code {
	switch {
	case stat >= 200 && stat < 300:
		return codes.OK
	case stat >= 400 && stat < 500:
		switch stat {
		case http.StatusBadRequest:
			return codes.InvalidArgument
		case http.StatusUnauthorized:
			return codes.Unauthenticated
		case http.StatusForbidden:
			return codes.PermissionDenied
		case http.StatusNotFound:
			return codes.NotFound
		case http.StatusMethodNotAllowed:
			return codes.InvalidArgument
		case http.StatusRequestTimeout:
			return codes.DeadlineExceeded
		case http.StatusConflict:
			return codes.Aborted
		case http.StatusRequestedRangeNotSatisfiable:
			return codes.OutOfRange
		case http.StatusLocked:
			return codes.Aborted
		case http.StatusPreconditionFailed, http.StatusExpectationFailed:
			return codes.FailedPrecondition
		case http.StatusTooManyRequests:
			return codes.ResourceExhausted
		case 499:
			return codes.Canceled
		default:
			return codes.InvalidArgument
		}
	case stat >= 500 && stat < 600:
		switch stat {
		case http.StatusInternalServerError:
			return codes.Internal
		case http.StatusNotImplemented:
			return codes.Unimplemented
		case http.StatusBadGateway:
			return codes.Unknown
		case http.StatusGatewayTimeout:
			return codes.DeadlineExceeded
		case http.StatusServiceUnavailable:
			return codes.Unavailable
		default:
			return codes.Internal
		}
	default:
		// 1XX (not supported by GRPC), 3xx/redirects (not supported by GRPC), other codes
		return codes.Unknown
	}
}
