package sse

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func isSameError(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}

func TestDecode(t *testing.T) {
	type Expected struct {
		Event *Event
		Error error
	}

	tests := []struct {
		Name     string
		Input    string
		Expected []Expected
	}{
		{
			Name:  "empty",
			Input: "",
			Expected: []Expected{
				{
					Error: io.EOF,
				},
			},
		},
		{
			Name: "single event",
			Input: strings.Join([]string{
				"data: hello",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Event: &Event{
						Type: "message",
						Data: []byte("hello"),
					},
				},
			},
		},
		{
			Name: "multiple events",
			Input: strings.Join([]string{
				"data: hello",
				"",
				"data: world",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Event: &Event{
						Type: "message",
						Data: []byte("hello"),
					},
				},
				{
					Event: &Event{
						Type: "message",
						Data: []byte("world"),
					},
				},
			},
		},
		{
			Name: "data crossing multiple lines",
			Input: strings.Join([]string{
				"data: hello",
				"data: world",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Event: &Event{
						Type: "message",
						Data: []byte("helloworld"),
					},
				},
			},
		},

		{
			Name: "event type",
			Input: strings.Join([]string{
				"event: trailer",
				"data: hello",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Event: &Event{
						Type: "trailer",
						Data: []byte("hello"),
					},
				},
			},
		},
		{
			Name: "empty data",
			Input: strings.Join([]string{
				"data: ",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Event: &Event{
						Type: "message",
						Data: []byte(""),
					},
				},
			},
		},
		{
			Name: "empty event type",
			Input: strings.Join([]string{
				"event: ",
				"data: hello",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Event: &Event{
						Type: "message",
						Data: []byte("hello"),
					},
				},
			},
		},
		{
			Name: "missing field name",
			Input: strings.Join([]string{
				"foo",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Error: errors.New("malformed event"),
				},
			},
		},
		{
			Name: "unsupported field name",
			Input: strings.Join([]string{
				"foo: bar",
				"",
			}, "\n"),
			Expected: []Expected{
				{
					Error: errors.New("malformed event"),
				},
			},
		},
		{
			Name: "unexpected end of input",
			Input: strings.Join([]string{
				"data: hello",
			}, "\n"),
			Expected: []Expected{
				{
					Error: io.ErrUnexpectedEOF,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			decoder := NewDecoder(strings.NewReader(test.Input))

			for _, item := range test.Expected {
				event, err := decoder.Decode()

				if !reflect.DeepEqual(event, item.Event) {
					t.Fatalf("unexpected event: %v; expecting %v", event, item.Event)
				}

				if !isSameError(err, item.Error) {
					t.Fatalf("unexpected error: %v; expecting %v", err, item.Error)
				}
			}
		})
	}
}
