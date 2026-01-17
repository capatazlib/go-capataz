package s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRestartBackoff(t *testing.T) {
	for _, tc := range []struct {
		desc         string
		base         time.Duration
		max          time.Duration
		restartCount uint32
		result       time.Duration
	}{
		{
			desc:         "empty",
			max:          0,
			base:         0,
			restartCount: 0,

			result: 0,
		},
		{
			desc:         "zero",
			max:          time.Hour,
			base:         10 * time.Millisecond,
			restartCount: 0,

			result: 0,
		},
		{
			desc:         "one",
			max:          time.Hour,
			base:         10 * time.Millisecond,
			restartCount: 1,

			result: 10 * time.Millisecond,
		},
		{
			desc:         "two",
			max:          time.Hour,
			base:         10 * time.Millisecond,
			restartCount: 2,

			result: 20 * time.Millisecond,
		},
		{
			desc:         "three",
			max:          time.Hour,
			base:         10 * time.Millisecond,
			restartCount: 3,

			result: 40 * time.Millisecond,
		},
		{
			desc:         "max",
			max:          234 * time.Millisecond,
			base:         10 * time.Millisecond,
			restartCount: 30,

			result: 234 * time.Millisecond,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			rb := restartBackoff{max: tc.max, base: tc.base}
			result := rb.duration(tc.restartCount)
			require.Equal(t, tc.result, result, result.String())
		})
	}
}
