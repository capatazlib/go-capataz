package c

import (
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

func withStacktrace(err error) error {
	return &errorWithKVs{
		error: err,
		kvs: map[string]interface{}{
			"stacktrace": string(debug.Stack()),
		},
	}
}
