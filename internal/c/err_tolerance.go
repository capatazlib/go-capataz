package c

import (
	"time"
)

// ErrToleranceResult indicates the result of a error tolerance check
type ErrToleranceResult uint32

const (
	// ErrToleranceSurpassed indicates the error tolerance has been surpassed
	ErrToleranceSurpassed = iota
	// IncreaseErrCount indicates that we should allow the error to happen
	IncreaseErrCount
	// ResetErrCount indicates to reset the error count and time window
	ResetErrCount
)

func (etr ErrToleranceResult) String() string {
	switch etr {
	case ErrToleranceSurpassed:
		return "ErrToleranceSurpassed"
	case IncreaseErrCount:
		return "IncreaseErrCount"
	case ResetErrCount:
		return "ResetErrCount"
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

// Check verifies if the error tolerance has been reached with the given input values
func (et ErrTolerance) Check(restartCount uint32, createdAt time.Time) ErrToleranceResult {
	if createdAt == (time.Time{}) || et.isWithinErrorWindow(createdAt) {
		if et.MaxErrCount == 0 || et.didSurpassErrorCount(restartCount+1) {
			return ErrToleranceSurpassed
		}
		return IncreaseErrCount
	}
	return ResetErrCount
}
