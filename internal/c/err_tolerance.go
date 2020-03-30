package c

import "time"

type errToleranceResult uint32

const (
	errToleranceSurpassed = iota
	increaseErrCount
	resetErrCount
)

func (etr errToleranceResult) String() string {
	switch etr {
	case errToleranceSurpassed:
		return "errToleranceSurpassed"
	case increaseErrCount:
		return "increaseErrCount"
	case resetErrCount:
		return "resetErrCount"
	default:
		return "<Unknown errToleranceResult>"
	}
}

// ErrTolerance is a helper type that manages error tolerance logic
type ErrTolerance struct {
	MaxErrCount uint32
	ErrWindow   time.Duration
}

func (et ErrTolerance) isWithinErrorWindow(createdAt time.Time) bool {
	// when errWindow is 0, it means we never forget errors happened
	return time.Since(createdAt) < et.ErrWindow || et.ErrWindow == 0
}

func (et ErrTolerance) didSurpassErrorCount(restartCount uint32) bool {
	return et.MaxErrCount < restartCount
}

func (et ErrTolerance) check(restartCount uint32, createdAt time.Time) errToleranceResult {
	if et.isWithinErrorWindow(createdAt) {
		if et.didSurpassErrorCount(restartCount + 1) {
			return errToleranceSurpassed
		}
		return increaseErrCount
	}
	return resetErrCount
}
