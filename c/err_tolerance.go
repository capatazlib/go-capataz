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

type errTolerance struct {
	maxErrCount uint32
	errWindow   time.Duration
}

func (et errTolerance) isWithinErrorWindow(createdAt time.Time) bool {
	// when errWindow is 0, it means we never forget errors happened
	return time.Since(createdAt) < et.errWindow || et.errWindow == 0
}

func (et errTolerance) didSurpassErrorCount(restartCount uint32) bool {
	return et.maxErrCount < restartCount
}

func (et errTolerance) check(restartCount uint32, createdAt time.Time) errToleranceResult {
	if et.isWithinErrorWindow(createdAt) {
		if et.didSurpassErrorCount(restartCount + 1) {
			return errToleranceSurpassed
		}
		return increaseErrCount
	}
	return resetErrCount
}
