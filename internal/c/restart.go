package c

func (ch Child) assertErrorTolerance(err error) (uint32, *ErrorToleranceReached) {
	errTolerance := ch.spec.ErrTolerance
	switch errTolerance.check(ch.restartCount, ch.createdAt) {
	case errToleranceSurpassed:
		return 0, &ErrorToleranceReached{
			failedChildName:        ch.GetRuntimeName(),
			failedChildErrCount:    errTolerance.MaxErrCount,
			failedChildErrDuration: errTolerance.ErrWindow,
			err:                    err,
		}
	case increaseErrCount:
		return ch.restartCount + uint32(1), nil
	case resetErrCount:
		// not zero given we need to account for the error that just happened
		return uint32(1), nil
	default:
		panic("Invalid implementation of errTolerance values")
	}
}

// Restart spawns a new Child and keeps track of the restart count.
func (ch Child) Restart(
	supParentName string,
	supNotifyCh chan<- ChildNotification,
	wasComplete bool,
	prevErr error,
) (Child, error) {
	chSpec := ch.GetSpec()

	var newCh Child
	var startErr error

	if wasComplete {
		newCh, startErr = chSpec.DoStart(supParentName, supNotifyCh)
		if startErr != nil {
			return Child{}, startErr
		}
	} else {
		restartCount, toleranceErr := ch.assertErrorTolerance(prevErr)
		if toleranceErr != nil {
			return Child{}, toleranceErr
		}
		newCh, startErr = chSpec.DoStart(supParentName, supNotifyCh)
		if startErr != nil {
			return Child{}, startErr
		}
		newCh.restartCount = restartCount
	}

	return newCh, nil
}
