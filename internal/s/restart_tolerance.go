package s

import (
	"time"
)

// restartToleranceResult indicates the result of a error tolerance check
type restartToleranceResult uint32

const (
	// restartToleranceSurpassed indicates the error tolerance has been surpassed
	restartToleranceSurpassed = iota
	// incRestartCount indicates that we should allow the error to happen
	incRestartCount
	// resetRestartCount indicates to reset the error count and time window
	resetRestartCount
)

func (rtr restartToleranceResult) String() string {
	switch rtr {
	case restartToleranceSurpassed:
		return "restartToleranceSurpassed"
	case incRestartCount:
		return "incRestartCount"
	case resetRestartCount:
		return "resetRestartCount"
	default:
		return "<Unknown restartToleranceResult>"
	}
}

// restartTolerance is a helper type that manages error tolerance logic
type restartTolerance struct {
	MaxRestartCount uint32
	RestartWindow   time.Duration
}

func (rt restartTolerance) isWithinRestartWindow(createdAt time.Time) bool {
	// when errWindow is 0, it means we never forget errors happened
	return time.Since(createdAt) < rt.RestartWindow || rt.RestartWindow == 0
}

func (rt restartTolerance) didSurpassMaxRestartCount(restartCount uint32) bool {
	return rt.MaxRestartCount < restartCount
}

// check verifies if the error tolerance has been reached with the given input values
func (rt restartTolerance) check(restartCount uint32, createdAt time.Time) restartToleranceResult {
	if createdAt == (time.Time{}) || rt.isWithinRestartWindow(createdAt) {
		if rt.MaxRestartCount == 0 || rt.didSurpassMaxRestartCount(restartCount+1) {
			return restartToleranceSurpassed
		}
		return incRestartCount
	}
	return resetRestartCount
}

type restartBackoff struct {
	base time.Duration
	max  time.Duration
}

func (rb restartBackoff) duration(restartCount uint32) time.Duration {
	if rb.base == 0 {
		return 0
	}
	dur := time.Duration(1 << (restartCount - 1))
	dur *= rb.base
	if dur > rb.max {
		return rb.max
	}
	return dur
}
