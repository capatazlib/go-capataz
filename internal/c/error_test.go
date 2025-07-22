package c

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func boomErr() (ret error) {
	defer func() {
		p := recover()
		err := fmt.Errorf("%v", p)
		ret = withStacktrace(err)
	}()
	panic("boom!")
}

func TestWithStacktrace(t *testing.T) {
	require := require.New(t)

	err := boomErr()
	require.Equal("boom!", err.Error())
	kvs := err.(interface{ KVs() map[string]interface{} }).KVs()
	require.Contains(kvs["stacktrace"], "c/error_test.go:16")
	require.Contains(kvs["stacktrace"], "internal/c.boomErr")
	require.Contains(kvs["stacktrace"], "\n")
}
