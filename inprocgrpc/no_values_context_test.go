package inprocgrpc

import (
	"testing"

	"golang.org/x/net/context"
)

func TestNoValuesContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "abc", "def")
	ctx = context.WithValue(ctx, "xyz", "123")
	ctx = context.WithValue(ctx, "foo", "bar")
	ctx, cancel := context.WithCancel(ctx)

	nvCtx := context.Context(noValuesContext{ctx})
	nvCtx = context.WithValue(nvCtx, "frob", "nitz")

	// make sure no values are supplied by wrapped context
	if nvCtx.Value("abc") != nil {
		t.Errorf(`noValuesContext should not have value for key "abc"`)
	}
	if nvCtx.Value("xyz") != nil {
		t.Errorf(`noValuesContext should not have value for key "xyz"`)
	}
	if nvCtx.Value("foo") != nil {
		t.Errorf(`noValuesContext should not have value for key "foo"`)
	}
	// it should, of course, have its own value
	if nvCtx.Value("frob") != "nitz" {
		t.Errorf(`noValuesContext returned wrong value for key "frob": expecting "nitz", got %v`, nvCtx.Value("frob"))
	}

	// and it still respect's cancellation/deadlines of the parent context
	if nvCtx.Err() != nil {
		t.Errorf(`noValuesContext should not be done!`)
	}
	cancel()
	if nvCtx.Err() != context.Canceled {
		t.Errorf(`noValuesContext should be canceled!`)
	}
}
