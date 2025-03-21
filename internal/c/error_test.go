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
		ret = withPanicLocation(err)
	}()
	panic("boom!")
}

func boomStr() (ret string) {
	defer func() {
		_ = recover()
		ret = panicLocation(0)
	}()
	panic("boom!")
}

func TestPanicLocation(t *testing.T) {
	require := require.New(t)
	str := boomStr()
	require.Contains(str, "c/error_test.go:24")

	err := boomErr()
	require.Equal("boom!", err.Error())
	kvs := err.(interface{ KVs() map[string]interface{} }).KVs()
	require.Contains(kvs["panic.location"], "c/error_test.go:16")
	require.Contains(kvs["panic.location"], "internal/c.boom")
	require.NotContains(kvs["panic.location"], "\n")
	require.Contains(kvs["stacktrace"], "c/error_test.go:16")
	require.Contains(kvs["stacktrace"], "internal/c.boomErr")
	require.Contains(kvs["stacktrace"], "\n")
}
