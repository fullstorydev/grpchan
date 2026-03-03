package sse

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Event represents an SSE event. The decoder only supports the data and event fields.
type Event struct {
	Type string
	Data []byte
}

type Decoder struct {
	r *bufio.Reader
}

// NewDecoder creates a new SSE decoder. The decoder does not implement the full SSE specification.
// It only supports what we need, which only includes the data and event fields.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

func (d *Decoder) Decode() (*Event, error) {
	var current *Event
	for {
		line, err := d.r.ReadBytes('\n')
		line = bytes.TrimRight(line, "\r\n")

		if err == io.EOF {
			if len(line) != 0 {
				return nil, io.ErrUnexpectedEOF
			}
			if current != nil {
				if current.Type == "" {
					current.Type = "message"
				}
				return current, nil
			}
			return nil, io.EOF
		} else if err != nil {
			return nil, err
		}

		switch {
		case bytes.HasPrefix(line, []byte("data: ")):
			payload := line[6:]
			if current == nil {
				current = &Event{Data: payload}
			} else {
				current.Data = append(current.Data, payload...)
			}
		case bytes.HasPrefix(line, []byte("event: ")):
			if current == nil {
				current = &Event{}
			}
			current.Type = string(line[7:])
		case len(line) == 0:
			if current != nil {
				if current.Type == "" {
					current.Type = "message"
				}
				return current, nil
			}
		default:
			return nil, errors.New("malformed event")
		}
	}
}

type Encoder struct {
	w io.Writer
}

// NewEncoder creates a new SSE encoder. The encoder does not implement the full SSE specification.
// It only supports what we need, which only includes the data and event fields.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(event *Event) error {
	if event.Type == "" || event.Type == "message" {
		_, err := fmt.Fprintf(e.w, "data: %s\n\n", event.Data)
		return err
	}

	_, err := fmt.Fprintf(e.w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
	return err
}
