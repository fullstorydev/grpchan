package inprocgrpc

import (
	"fmt"
	"reflect"

	"google.golang.org/grpc/encoding"

	"github.com/fullstorydev/grpchan/internal"
)

// Cloner knows how to make copies of messages. It can be asked to copy one
// value into another, and it can also be asked to simply synthesize a new
// value that is a copy of some input value.
//
// This is used to copy messages between in-process client and server. Copying
// will usually be more efficient than marshalling to bytes and back (though
// that is a valid strategy that a custom Cloner implementation could take).
// Copies are made to avoid sharing values across client and server goroutines.
type Cloner interface {
	Copy(out, in interface{}) error
	Clone(interface{}) (interface{}, error)
}

// ProtoCloner is the default cloner used by an in-process channel. This
// implementation can correctly handle protobuf messages. Copy and clone
// operations will fail if the input message is not a protobuf message (in
// which case a custom cloner must be used).
type ProtoCloner struct{}

var _ Cloner = ProtoCloner{}

func (ProtoCloner) Copy(out, in interface{}) error {
	return internal.CopyMessage(out, in)
}

func (ProtoCloner) Clone(in interface{}) (interface{}, error) {
	return internal.CloneMessage(in)
}

// CloneFunc adapts a single clone function to the Cloner interface. The given
// function implements the Clone method. To implement the Copy method, the given
// function is invoked and then reflection is used to shallow copy the clone to
// the output.
func CloneFunc(fn func(interface{}) (interface{}, error)) Cloner {
	copyFn := func(out, in interface{}) error {
		in, err := fn(in) // deep copy input
		if err != nil {
			return err
		}

		// then shallow-copy into out via reflection
		src := reflect.Indirect(reflect.ValueOf(in))
		dest := reflect.Indirect(reflect.ValueOf(out))
		if src.Type() != dest.Type() {
			return fmt.Errorf("incompatible types: %v != %v", src.Type(), dest.Type())
		}
		if !dest.CanSet() {
			return fmt.Errorf("unable to set destination: %v", reflect.ValueOf(out).Type())
		}
		dest.Set(src)
		return nil

	}
	return &funcCloner{clone: fn, copy: copyFn}
}

// CopyFunc adapts a single copy function to the Cloner interface. The given
// function implements the Copy method. To implement the Clone method, a new
// value of the same type is created using reflection and then the given
// function is used to copy the input to the newly created value.
func CopyFunc(fn func(out, in interface{}) error) Cloner {
	cloneFn := func(in interface{}) (interface{}, error) {
		clone := reflect.New(reflect.TypeOf(in).Elem()).Interface()
		if err := fn(clone, in); err != nil {
			return nil, err
		}
		return clone, nil
	}
	return &funcCloner{clone: cloneFn, copy: fn}
}

// CodecCloner uses the given codec to implement the Cloner interface. The Copy
// method is implemented by using the code to marshal the input to bytes and
// then unmarshal from bytes into the output value. The Clone method then uses
// reflection to create a new value of the same type and uses this strategy to
// then copy the input to the newly created value.
func CodecCloner(codec encoding.Codec) Cloner {
	return CopyFunc(func(out, in interface{}) error {
		if b, err := codec.Marshal(in); err != nil {
			return err
		} else if err := codec.Unmarshal(b, out); err != nil {
			return err
		}
		return nil
	})
}

type funcCloner struct {
	clone func(interface{}) (interface{}, error)
	copy  func(in, out interface{}) error
}

var _ Cloner = (*funcCloner)(nil)

func (c *funcCloner) Copy(out, in interface{}) error {
	return c.copy(out, in)
}

func (c *funcCloner) Clone(in interface{}) (interface{}, error) {
	return c.clone(in)
}
