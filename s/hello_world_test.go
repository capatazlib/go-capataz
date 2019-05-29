package s_test

import (
	"testing"

	sut "github.com/capatazlib/go-capataz/s"
	"github.com/stretchr/testify/require"
)

func TestHelloWorld(t *testing.T) {
	require.Equal(t, "Hello World\n", sut.Hello("World"), "Not Equals")
}
