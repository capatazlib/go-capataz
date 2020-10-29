/*
Package cap offers an API to model applications as an static supervision tree of
goroutines, with monitoring rules that follow semantics from Erlang's OTP
Library.

This implementation allows the creation of a root supervisor, sub-trees and
workers; it also offers various error monitoring/restart strategies and start
and shutdown ordering guarantees.

Why Supervision Trees

Goroutines are a great tool to solve many jobs, but, alone, they come with a few
drawbacks:

* We don't have easy ways to know when a goroutine stopped working (e.g. random
panic) other than the main program crashing.

* As soon as we spawn a group of goroutines, we don't have any guarantees they
will start in the order we anticipated.

* The way goroutines are designed make it very easy to not follow golang best
practices like message passing; instead, it is easy to reach mutexes.

Granted, we can use tools like gochans, sync.WaitGroup, sync.Cond, etc. to deal
with these issues, but, they are very low-level and there is unnecessary
boilerplate involved in order to make them work.

Capataz offers a declarative API that allows developers to easily manage
goroutine error handling and start/stop system mechanisms through type
composition.

A great benefit, is that you can compose several sub-systems together into a
bigger supervised system, increasing the reliability of your application
altogether.

Capataz offers a few types and functions that you need to learn upfront to be
effective with it.

Worker

A worker is equivalent to a Goroutine. You create a worker using the NewWorker
function

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
		// (4)
		cap.WithShutdown(cap.Timeout(1 * time.Second)),
	})

The first argument (1) is a name. This name is used for metadata/tracing
purposes and it will be very handy when monitoring your application behavior.

The second argument (2) is the function you want to run on a goroutine.

This function receives a context.Context record; you must use this context to
check for stop signals that may get triggered when failures are detected, and
add cleanup code if you allocated some resource.

The function also returns an error, which may indicate that the worker goroutine
finished on a bad state. Depending on your setup, an error may indicate a
supervisor that the worker goroutine needs to get restarted.

The third argument (3) is a WorkerOpt. This option in particular indicates that
the worker goroutine should always get restarted, whether the function returns
an error or returns nil. You can check the WithRestart function for more
available options.

The fourth argument (4) is also a WorkerOpt. This option specifies that the
supervisor must wait at least 1 second to stop "my-worker" before giving up.

All starts and stops of Worker goroutines are managed by its Supervisor, so you
need to plug this worker into a SupervisorSpec in order to see it in action.

SupervisorSpec

A supervisor spec is a specification of a supervision (sub-)tree. It is
responsible for creating the Supervisor that will start, restart, and stop all
the specified workers or sub-trees.

You can create a SupervisorSpec using the NewSupervisorSpec function

	root := cap.NewSupervisorSpec(
		// (1)
		"my-supervisor",
		// (2)
		cap.WithNodes(myWorker, myOtherWorker, cap.Subtree(mySubsystem)),
		// (3)
		cap.WithStartOrder(cap.LeftToRight),
		// (4)
		cap.WithStrategy(cap.OneForOne),
	)

The first argument (1) is a name. Like in NewWorker call, this name is going to
be used for monitoring/tracing purposes.

The second argument (2) is a function that returns a pre-defined BuildNodesFn,
the function describes all the nodes this supervisor must monitor. We have two
workers (myWorker, myOtherWorker) and one subtree (mySubsystem), which is
another supervisor that monitors other workers.

The third argument (3) is an Opt. This option specifies how these tree nodes
should get started. the LeftToRight value indicates that when starting, it
should start with myWorker, then myOtherWorker and end with mySubsystem.

This API will guarantee that the mySubsystem sub-tree won't start until
myOtherWorker starts, and that one won't start until myWorker starts. When root
shuts down, it will stop each child node in reverse order, starting with
mySubsystem, then myOtherWorker and finally myWorker.

The fourth argument (4) is an Opt. This option specifies how children should get
restarted when one of them fails. In this particular example, only the failing
child node will get restarted. Check the WithStrategy docs for more details.

Once we have a SupervisorSpec, we have the blueprint to start a system, spawning
each goroutine we need in order. Note that all the wiring of resources is
happening at build time. The actual allocation of resources comes at later step.

We will use this SupervisorSpec to create a Supervisor value.

Supervisor

To create Supervisor, you need to call the Start method of a SupervisorSpec
value

	sup, startErr := root.Start()

This call will return a Supervisor record or an error. The error may happen when
one of the supervision tree children nodes is not able to allocate some IO
resource and reports it failed to Start (see NewWithStart and NewSupervisorSpec
for details).

The Start call is synchronous, the program will not continue until all worker
nodes and sub-trees are started in the specified order. With the SupervisorSpec
example above the following supervision tree will be spawned

	root
	|
	` root/my-worker (spawned goroutine)
	|
	` root/my-other-worker (spawned goroutine)
	|
	 ` my-subsystem
		|
		` <other workers or sub-trees here>

At this point, all your program workers should have allocated their resources
and running as expected. If you want to stop the system, you can call the Stop
method on the returned supervisor:

	stopErr := sup.Stop()

The stopErr error will tell you if any of the children failed to stop. You may
also join the main goroutine with the root supervisor monitor goroutine using
the Wait method

	supErr := sup.Wait()

Resources in Workers

There are times when an IO resource belongs to a single worker, and it doesn't
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
				return runDBWorker(ctx, db, reportCh)
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
accessing the database, so it is responsible for initializing it. It uses the
notifyStart function to indicate if there was an error on start when connecting
to the database. If this happened (1), the start procedure of the root
supervisor will stop and abort, if not, the Start routine won't continue until
the worker notifies is ready (2).

What about that channel though?

Where do we create this channel that this producer and possibly a consumer use
to communicate?

For this, we need to manage resources in sub-trees.

Resources in Sub-trees

For the example above, we could create the channel at the top-level of our
application (main function) and pass them as arguments there; however, there is
a drawback with doing it that way. If for some reason the channel gets closed,
you get a worker that is always going to fail, until the error tolerance of all
the supervisors is met (hopefully) and it crashes hard.

Another approach is to have a subtree create the channel at the same level as
the worker and producer (which the subtree supervisor will be monitoring). That
way, if for some reason, the subtree supervisor fails, it will create a new
gochan and restart the producer and the consumer.

To do this, we need to define a custom BuildNodesFn in our NewSupervisorSpec
call.

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
			cap.WithStartOrder(cap.LeftToRight),
		)

	}

On the code above, instead of using the cap.WithNodes function, we define our
own BuildNodesFn (1). This function creates a gochan for Report values that both
producer and consumer are going to use (2).

We then declare a cleanup function that closes the allocated gochan in case this
supervisor gets restarted or stopped (3), and finally, it returns the nodes that
the supervisor is going to monitor (4), the cleanup function and a nil error.

If there is a restart of this supervisor, the cleanup function gets called, and
the logic inside the BuildNodesFn gets called again, restarting this specific
sub-system and without affecting other parts of the bigger system.

Other Examples

Check the examples directory in the Github repository for more examples

https://github.com/capatazlib/go-capataz/tree/master/examples


*/
package cap
