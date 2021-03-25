package c

////////////////////////////////////////////////////////////////////////////////

// GetName returns the specified name for a Child Spec
func (chSpec ChildSpec) GetName() string {
	return chSpec.Name
}

// Terminate is a synchronous procedure that halts the execution of the child.
// The first return value is false if the worker is already terminated. The
// second return value is non-nil when the child fails to terminate. If the
// first return value is true, the second return value will always be nil.
func (ch Child) Terminate() (bool, error) {
	ch.cancel()
	return ch.wait(ch.spec.Shutdown)
}
