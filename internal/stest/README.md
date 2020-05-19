## How tests work?

Capataz' supervision system was built with observability as a prime directive,
given this, the library emit events for every goroutine that starts, stops,
terminates and fails. To make sure this notification is accurate, all the test
infrastructure is built around making sure that the events are triggered in the
expected order when an specific set of children are used in a supervision
system.

Each test is composed by a few components:

### `Child` builders

The different known children builders are:

* `WaitDoneWorker`
* `FailStartWorker`
* `FailOnSignalWorker`
* `NeverTerminateWorker`

All of them always return a `ChildSpec` with some expected behavior. In some
cases a child builder returns a second value that allows you to control when the
child _really_ "starts" executing. This is important because we want to build an
scenario that we can assert without relying on time delays, and ensuring the
behavior triggers when things are setup, is a good way to have reliable
test-suite.

### `ObserveSupervisor` call

Once we have defined a tree to test, we use the `ObserveSupervisor` function to
start the supervisor tree. This function will collect in memory all the events
that are emitted by the notification system embedded in the library.

#### Notes about the callback parameter

The function receives a callback that gets executed once the supervision tree is
up; this is the best spot to start the controlled execution of children that you
created in the supervision tree (start failing children, etc).

The first parameter of the callback is an `EventManager` record, this has an
`Iterator` method that returns a record that has functions (`SkipTill`,
`TakeTill`, etc.) that allow you to wait for certain events to happen (using an
assertion predicate). This approach is necessary to avoid race-conditions, you
want to make sure an event happens before triggering a new one concurrently.

Finally, this function returns two values, the events that got triggered, and if
the supervisor failed with an error. You can then use the assertion functions
(described bellow) to check that the events are the ones you expect.

### Assertion functions

The different known assert functions are:

* `AssertExactMatch`
* `AssertPartialMatch`

These functions receive a list of predicate functions as parameters, they allow
you to assert that the returned collected events match all the criteria that you
are expecting.

Some of the predicate methods are:

* `SupervisorStarted`
* `WorkerStarted`
* `SupervisorStartFailed`
* `WorkerStartFailed`
* `SupervisorFailed`
* `WorkerFailed`
* `SupervisorTerminated`
* `WorkerTerminated`

Check the test implementation to see how they are used.

#### A note about event order in assertions

Some may find counter-intuitive the order of the collected events. The
supervision tree start procedure will spawn goroutines for the leafs of the tree
first. Every leave of the tree correspond to a Worker goroutine, and every
branch corresponds to a supervisor that tracks errors on its children.

A supervisor is considered in a _running_ state when all its children goroutines
are spawned. You may be wondering, what happens to errors triggered by
goroutines while a supervisor is bootstraping? Each worker goroutine will block
trying to send an error to it's supervisor monitor; once the supervisor is in a
running state, the errors are handled as expected.
