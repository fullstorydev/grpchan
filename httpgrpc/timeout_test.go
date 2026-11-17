package httpgrpc

import (
	"math"
	"net/http"
	"testing"
	"time"
)

// TestEncodeTimeout checks that a client's timeout is sent in a form that the
// server accepts, which allows at most 8 digits, and that it loses no more
// than the precision of the unit it is sent in.
func TestEncodeTimeout(t *testing.T) {
	testCases := []struct {
		timeout time.Duration
		want    string
	}{
		{timeout: time.Nanosecond, want: "1n"},
		{timeout: 99999999 * time.Nanosecond, want: "99999999n"},
		{timeout: 100 * time.Millisecond, want: "100000u"},
		{timeout: 30 * time.Second, want: "30000000u"},
		{timeout: 48 * time.Hour, want: "172800S"},
		{timeout: 30000 * time.Hour, want: "1800000M"},
		{timeout: math.MaxInt64, want: "2562047H"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.timeout.String(), func(t *testing.T) {
			got := encodeTimeout(testCase.timeout)
			if got != testCase.want {
				t.Fatalf("encodeTimeout: got %q, want %q", got, testCase.want)
			}
			decoded, ok, err := timeoutFromHeaders(http.Header{"Grpc-Timeout": []string{got}})
			if err != nil || !ok {
				t.Fatalf("server can't read %q: ok %v, err %v", got, ok, err)
			}
			unitSize := map[byte]time.Duration{
				'n': time.Nanosecond, 'u': time.Microsecond, 'm': time.Millisecond,
				'S': time.Second, 'M': time.Minute, 'H': time.Hour,
			}[got[len(got)-1]]
			if decoded > testCase.timeout || testCase.timeout-decoded > unitSize {
				t.Fatalf("server read %q as %v, want within %v below %v", got, decoded, unitSize, testCase.timeout)
			}
		})
	}
}
