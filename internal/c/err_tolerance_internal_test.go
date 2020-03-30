package c

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestErrTolerance(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		maxErrCount uint32
		errWindow   time.Duration
		errCount    uint32
		createdAt   time.Time
		result      errToleranceResult
	}{
		{
			desc: "zero window tolerance means we never forget errors",
			// spec: two errors on a zero window
			maxErrCount: 2,
			errWindow:   0,
			// input
			errCount:  2,
			createdAt: time.Now(),

			result: errToleranceSurpassed,
		},
		{
			desc: "when err count is >= than maxErrCount after window has passed",
			// spec: two errors on a zero window
			maxErrCount: 2,
			errWindow:   5 * time.Second,
			// input
			errCount:  2,
			createdAt: time.Now().Add(time.Duration(-6) * time.Second), /* 6 seconds ago */

			result: resetErrCount,
		},
		{
			desc: "when err count is <= than maxErrCount whithin err window count",
			// spec: two errors on a zero window
			maxErrCount: 2,
			errWindow:   5 * time.Second,
			// input
			errCount:  1,
			createdAt: time.Now().Add(time.Duration(4) * time.Second), /* 4 seconds ago */

			result: increaseErrCount,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			et := ErrTolerance{MaxErrCount: tc.maxErrCount, ErrWindow: tc.errWindow}
			result := et.check(tc.errCount, tc.createdAt)
			require.True(t, tc.result == result, result.String())
		})
	}
}
