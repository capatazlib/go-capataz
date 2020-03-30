package c

func (ch Child) assertErrorTolerance() (uint32, *ErrorToleranceReached) {
	errTolerance := ch.spec.ErrTolerance
	switch errTolerance.check(ch.restartCount, ch.createdAt) {
	case errToleranceSurpassed:
		return 0, &ErrorToleranceReached{
			failedChildName:        ch.GetRuntimeName(),
			failedChildErrCount:    errTolerance.MaxErrCount,
			failedChildErrDuration: errTolerance.ErrWindow,
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
) (Child, error) {

	restartCount, toleranceErr := ch.assertErrorTolerance()
	if toleranceErr != nil {
		return Child{}, toleranceErr
	}

	chSpec := ch.GetSpec()
	newChild, startErr := chSpec.DoStart(supParentName, supNotifyCh)
	if startErr != nil {
		return Child{}, startErr
	}

	newChild.restartCount = restartCount
	return newChild, nil
}
