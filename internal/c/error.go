package c

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

type errorWithKVs struct {
	error
	kvs map[string]interface{}
}

type kvser interface {
	KVs() map[string]interface{}
}

func (err *errorWithKVs) KVs() map[string]interface{} {
	if inner, ok := err.error.(kvser); ok {
		kvs := make(map[string]interface{})
		for k, v := range inner.KVs() {
			kvs[k] = v
		}
		for k, v := range err.kvs {
			kvs[k] = v
		}
		return kvs
	}

	return err.kvs
}

func (err *errorWithKVs) GetKVs() map[string]interface{} {
	return err.KVs()
}

func (err *errorWithKVs) Unwrap() error {
	return err.error
}

func withPanicLocation(err error) error {
	return &errorWithKVs{
		error: err,
		kvs: map[string]interface{}{
			"panic.location": panicLocation(1),
			"stacktrace":     string(debug.Stack()),
		},
	}
}

// panicLocation will, when called from within a `defer` when `recover() !=
// nil`, will return the location of the panic. If called from another function,
// increment `skip` to account for extra stack frames.
func panicLocation(skip int) string {
	pc, file, line, ok := runtime.Caller(skip + 3)
	if !ok {
		return "unknown"
	}

	fc := runtime.FuncForPC(pc)
	if fc == nil {
		return "unknown"
	}

	return fmt.Sprintf("%s:%d %s", file, line, fc.Name())
}
