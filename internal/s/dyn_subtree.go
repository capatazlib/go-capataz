package s

import (
	"context"

	"github.com/capatazlib/go-capataz/internal/c"
)

// Spawner is a record that can spawn other workers, and can wait
// for termination
type Spawner interface {
	Spawn(Node) (func() error, error)
}

type spawnerClient struct {
	ctrlChan chan ctrlMsg
}

func newSpawnerClient(ctrlChan chan ctrlMsg) spawnerClient {
	return spawnerClient{ctrlChan: ctrlChan}
}

func (s spawnerClient) Spawn(node Node) (func() error, error) {
	return sendSpawnToSupervisor(s.ctrlChan, node)
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
	opts ...c.Opt,
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
	runFn func(context.Context, NotifyStartFn, Spawner) error,
	spawnerOpts []Opt,
	opts ...c.Opt,
) Node {
	return func(supSpec SupervisorSpec) c.ChildSpec {
		dynSubtreeSpec := NewSupervisorSpec(
			name,
			func() ([]Node, CleanupResourcesFn, error) {
				// we cannot create the dynamic supervisor here because
				// we don't have a context to build it with. We are going
				// to create a promise of a value so that the worker has access
				// to it later
				ctrlChan := make(chan ctrlMsg)

				spawnerSpec := NewSupervisorSpec("spawner", WithNodes(), spawnerOpts...)
				spawnerNode := func(parent SupervisorSpec) c.ChildSpec {
					return parent.subtree(spawnerSpec, ctrlChan, opts...)
				}

				cleanup := func() error { return nil }

				return []Node{
					spawnerNode,
					NewWorkerWithNotifyStart(
						"worker",
						func(ctx context.Context, notifyStart NotifyStartFn) error {
							// we create a value that allows us to communicate with the
							// spawner supervision sub-tree in a safe way.
							spawner := newSpawnerClient(ctrlChan)
							return runFn(ctx, notifyStart, spawner)
						},
						opts...,
					),
				}, cleanup, nil
			},
			// if the dynamic sub-tree or the spawner node fail, restart both of them
			WithStrategy(OneForAll),
		)

		ctrlChan := make(chan ctrlMsg)
		return supSpec.subtree(dynSubtreeSpec, ctrlChan)
	}
}
