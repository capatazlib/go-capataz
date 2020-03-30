package c

////////////////////////////////////////////////////////////////////////////////

// GetName returns the specified name for a Child Spec
func (chSpec ChildSpec) GetName() string {
	return chSpec.Name
}

// Stop is a synchronous procedure that halts the execution of the child
func (ch Child) Stop() error {
	ch.cancel()
	return ch.wait(ch.spec.Shutdown)
}
