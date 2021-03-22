package c

////////////////////////////////////////////////////////////////////////////////

// GetName returns the specified name for a Child Spec
func (chSpec ChildSpec) GetName() string {
	return chSpec.Name
}

// Terminate is a synchronous procedure that halts the execution of the child. it returns an
// error if the child fails to terminate. The second return value is false if the worker is
// already terminated.
func (ch Child) Terminate() (bool, error) {
	ch.cancel()
	return ch.wait(ch.spec.Shutdown)
}
