/*
Package cap offers an API to model applications as an static supervision tree of
goroutines, with monitoring rules that follow semantics from Erlang's OTP
Library.

This implementation allows the creation of a root supervisor, sub-trees and
workers; it also offers various error monitoring/restart strategies and start
and shutdown ordering guarantees.

Why Supervision Trees

Goroutines are a great tool to solve many jobs, but they come with a few
drawbacks:

* We don't easy ways to know a goroutine stopped working (e.g. random panic)

* As soon as we spawn a group of goroutines, we don't have any guarantees they
will start in the order we anticipated.

* The way goroutines are designed make it very easy to not follow go mantras
like message passing; we usually reach mutexes often.

Of course, we can use tools like gochan, sync.WaitGroup, sync.Cond, etc. to deal
with a lot of these issues, but, they are very low-level.

Capataz offers a _declarative_ API that allows you to get goroutine error
handling and start/stop system mechanisms by enforcing good practices at the
type level.

What is great, you can compose many supervised systems together into a bigger
supervised system, increasing the reliability of your system altogether.

Capataz offers a few types and functions that you need to learn upfront to be
effective with it.

Worker

A worker is equivalent to a Goroutine, you can create a worker using the
NewWorker function

	myWorker := cap.NewWorker(
		// (1)
		"my-worker",
		// (2)
		func(ctx context.Context) error {
			select {
			case <-ctx.Done():
			// my bussiness logic here
			},
		// (3)
		cap.WithRestart(cap.Permanent),
		cap.WithShutdown(cap.Timeout(1 * time.Second)),
	})

The first argument (1) is a name, this name is used for metadata/tracing purposes
and it will be very handy when monitoring your application behavior.

The second argument (2) is the function you want to run on a goroutine.

This function receives a context.Context record; you must use this context to
check for stop signals that may get triggered when failures are detected, and
add cleanup code if you allocated some resource.

The function also returns an error, which may indicate that the worker goroutine
finished on a bad state. Depending on your worker setup, an error may indicate a
supervisor that you need to get restarted.

The third argument (3) is a WorkerOpt, this one in particular indicates that the
worker goroutine should always get restarted; when the function returns an error
or returns nil. You can check the WithRestart function for more available
options.

The fourth argument (4) is also a WorkerOpt, it specifies that the supervisor
must wait at least 1 second to stop "my-worker" before giving up.

All starts and stops of Worker node are managed in it's Supervisor, so you need
to plug this worker into a SupervisorSpec in order to see it in action.

SupervisorSpec

A supervisor spec is an specification of a supervision (sub-)tree. It is
responsible of creating the Supervisor that will start, restart, and stop all
the specified workers or sub-trees.

You can create a SupervisorSpec using the NewSupervisorSpec function

	root := cap.NewSupervisorSpec(
		// (1)
		"my-supervisor",
		// (2)
		cap.WithNodes(myWorker, myOtherWorker, cap.Subtree(mySubsystem)),
		// (3)
		cap.WithOrder(cap.LeftToRight),
		// (4)
		cap.WithStrategy(cap.OneForOne),
	)

The first argument (1) is a name, like the workers, this name is going to be used
for monitoring/tracing purposes.

The second argument (2) is a function that returns a pre-defined BuildNodesFn, this
function assigns all the nodes this supervisor must monitor. We have two workers
and one subtree, which is another supervisor that monitors other workers.

The third argument (3) is an Opt, this one specifies how these tree nodes should get
started. the LeftToRight value indicates that when starting, it should start
with myWorker, then myOtherWorker and end with mySubsystem.

This API will guarantee that the mySubsystem won't start until myOtherWorker
starts, and that one won't start until myWorker starts. When root shuts down, it
will stop each child node in reverse order, starting with mySubsystem, then
myOtherWorker and finally myWorker.

The fourth argument (4) is an Opt, this option specifies how children should get
restart when on of them fails. In this particular example, only the failing
child node will get restarted. Check the WithStrategy docs for more details.


Once we have a SupervisorSpec, we have the blueprint to start a system, spawning
each goroutine we need in order. Note all the wiring of resources is happening
at build time, the allocation resources comes at later step.

We will use this SupervisorSpec to create a Supervisor value.

Supervisor

To create Supervisor, you need to call the Start method of a SupervisorSpec
value

	sup, startErr := root.Start()

This call will return a Supervisor record or an error. The error may happen when
one of the children nodes is not able to allocate some IO resource and reports
it failed to Start (see NewWithStart and NewSupervisorSpec for details).

The Start call is synchronous, the program will not continue until all worker
nodes and sub-trees are started in the specified order. For our SupervisorSpec
above the following supervision tree will be spawned

	root
	|
	` root/my-worker (spawned goroutine)
	|
	` root/my-other-worker (spawned goroutine)
	|
	 ` my-subsystem
		|
		` <other workers or sub-trees here>

All your program should have allocated the resources and running as expected. If you want
to stop the system, you can call the Stop method

	stopErr := sup.Stop()

The stopErr error will tell you if any of the children failed to stop. You may
also join the main goroutine with the root supervisor monitor goroutine using
the Wait method

	supErr := sup.Wait()

Resources in Workers

There are times where an IO resource belongs to a single worker, and it doesn't
make sense to consider a worker "started" until this resource is allocated.

For this use case you can create a Worker goroutine that receives a callback
function that indicates that the Worker was started (or failed to start).

Lets use the NewWorkerWithNotifyStartFn function to model this behavior:

	func newReportGenWorker(
		name string,
		dbConfig dbConfig,
		reportCh chan Report,
		opts ...cap.WorkerOpt,
	) cap.Worker {
		return cap.NewWorkerWithNotifyStartFn(
			name,
			func(ctx context.Cotnext, notifyStart cap.NotifyStartFn) error {
				db, err := openDB(dbConfig)
				if err != nil {
					// (1)
					notifyStart(err)
					return err
				}
				// (2)
				notifyStart(nil)
				return runDBWorker(ctx, db)
			},
			opts...,
		)
	}

	// Look ma, no capataz dependencies
	func runDBWorker(ctx context.Context, db db.DB, reportCh chan Report) error {
		// Business Logic Here
	}


We have a Worker that is reponsible to send certain reports to a given channel
that some consumer is going to use. The Worker goroutine is the only one
accessing the database, so it is responsible of initializing it. It uses the
notifyStart function to indicate if there was an error on start when connecting
to the database. If this happened (1), the start proceduer of the supervision
system will stop and abort, if not, the Start routine won't continue until the
worker notifies is ready (2).

What about that channel though?

Where do we create this channel that this producer and possibly a consumer use
to communicate?

For this, we need to manage resources in sub-trees.

Resources in Sub-trees

For the example above, we could create the channel at the top-level of our
application (main function) and pass them as arguments there; however, there is
a drawback doing it that way. If for some reason the channel gets closed, you
get a worker that is always going to fail, until the error tolerance of all the
supervisors is met (hopefully) and it crashes hard.

Another approach, is to have the channel getting created in a subtree, and have
the worker and producer being monitored by it, if for some reason, the
supervisor fails, it will initialize a new gochan and restart the whole
operation. To do this, we need to defined a custom BuildNodesFn in our
NewSupervisorSpec call.

	func reportSubSystem(
		dbConfig dbConfig,
		dialConfig dialConfig,
	) cap.SupervisorSpec {

		return cap.NewSupervisorSpec(
			"reports-subsystem",
			// (1)
			func() ([]cap.Node, cap.CleanupResourcesFn, error) {
				// (2)
				reportCh := make(chan Report)

				// (3)
				cleanupFn : func() error {
					close(reportCh)
					return nil
				}

				// (4)
				return []cap.Node{
					newReportGenWorker(dbConfig, reportCh),
					newReportEmitter(dialConfig, reportCh),
				}, cleanupFn, nil
			},
			cap.WithStrategy(cap.OneForOne),
			cap.WithOrder(cap.LeftToRight),
		)

	}

On the code above, instead of using the WithNodes function, we define our own
BuildNodesFn (1). This function creates a gochan for Report that both producer and
consumer are going to use (2).

We then declare a cleanup function that closes the allocated gochan in case this
supervisor gets restarted or stopped (3), and finally, it returns the nodes that
is going to supervise (4).

If there is a restart of this supervisor, the cleanup function gets called, and
the logic inside the BuildNodesFn gets called again, restarting this part of
your bigger system without affecting other parts of the system.

Other Examples

Check the examples directory in the Github repository for more examples

https://github.com/capatazlib/go-capataz/tree/master/examples


*/
package cap
