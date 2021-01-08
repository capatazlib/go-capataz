package cap

import (
	"context"
	"fmt"
	"strings"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Spawner is a record that can spawn other workers, and can wait
// for termination
type Spawner interface {
	Spawn(Node) (func() error, error)
	Wait() error
}

// NewDynSubtree builds a worker that has receives a Spawner that allows it to
// create more child workers dynamically in a sub-tree.
//
// Note: The Spawner is automatically managed by the supervision tree, so
// clients are not required to terminate it explicitly.
//
func NewDynSubtree(
	name string,
	startFn func(context.Context, Spawner) error,
	spawnerOpts []Opt,
	opts ...WorkerOpt,
) Node {
	return NewDynSubtreeWithNotifyStart(
		name,
		func(ctx context.Context, notifyStart NotifyStartFn, spawner Spawner) error {
			notifyStart(nil)
			return startFn(ctx, spawner)
		},
		spawnerOpts,
		opts...,
	)
}

// NewDynSubtreeWithNotifyStart accomplishes the same goal as NewDynSubtree with
// the addition of passing an extra argument (notifyStart callback) to the
// startFn function parameter.
func NewDynSubtreeWithNotifyStart(
	name string,
	startFn func(context.Context, NotifyStartFn, Spawner) error,
	spawnerOpts []Opt,
	opts ...WorkerOpt,
) Node {
	return func(supSpec SupervisorSpec) c.ChildSpec {
		return c.NewWithNotifyStart(
			name,
			func(parentCtx context.Context, notifyChildStart c.NotifyStartFn) error {
				supRuntimeName, ok := c.GetNodeName(parentCtx)
				if !ok {
					return fmt.Errorf("library bug: subtree context does not have a name")
				}

				spawnerName := strings.Join([]string{supRuntimeName, "spawner"}, "/")

				spawnerOpts = append(spawnerOpts, WithNotifier(supSpec.eventNotifier))
				spawnerSup, spawnerSupErr := NewDynSupervisor(
					parentCtx, spawnerName, spawnerOpts...,
				)
				if spawnerSupErr != nil {
					notifyChildStart(spawnerSupErr)
					return spawnerSupErr
				}

				// ensure supervisor is terminated if startFn raises a panic.
				defer spawnerSup.Terminate()

				workerErr := startFn(parentCtx, notifyChildStart, &spawnerSup)

				// we can call Terminate multiple times as it is idempotent
				terminationErr := spawnerSup.Terminate()

				// when the spawner fails to terminate, we want to report the error of
				// this worker as a supervisor error
				if terminationErr != nil {
					nodeErrMap := map[string]error{}
					nodeErrMap[spawnerName] = terminationErr
					spawnerErr := &SupervisorTerminationError{
						supRuntimeName: supRuntimeName,
						nodeErrMap:     nodeErrMap,
						rscCleanupErr:  nil,
					}
					return spawnerErr
				}

				// otherwise, return the error as if you were a worker
				return workerErr
			},
			opts...,
		)
	}
}
