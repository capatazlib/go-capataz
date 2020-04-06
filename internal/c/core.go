package c

////////////////////////////////////////////////////////////////////////////////

// GetName returns the specified name for a Child Spec
func (chSpec ChildSpec) GetName() string {
	return chSpec.Name
}

// Terminate is a synchronous procedure that halts the execution of the child
func (ch Child) Terminate() error {
	ch.cancel()
	return ch.wait(ch.spec.Shutdown)
}
