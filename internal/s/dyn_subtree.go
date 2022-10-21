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
// NewDynSubtree builds a worker that receives a Spawner which allows it to
// create more child workers dynamically in a sibling sub-tree.
//
// # The runtime subtree is composed of a worker and a supervisor
//
// <name>
// |
// `- spawner (creates dynamic workers in sibling subtree)
// |
// `- subtree
//
//	|
//	`- <dynamic_worker>
//
// Note: The Spawner is automatically managed by the supervision tree, so
// clients are not required to terminate it explicitly.
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
				// this ctrlChan is going to be used by the subtree
				ctrlChan := make(chan ctrlMsg)

				spawnerSpec := NewSupervisorSpec("subtree", WithNodes(), spawnerOpts...)
				spawnerNode := func(parent SupervisorSpec) c.ChildSpec {
					return parent.subtree(spawnerSpec, ctrlChan, opts...)
				}

				cleanup := func() error {
					return nil
				}

				return []Node{
					spawnerNode,
					NewWorkerWithNotifyStart(
						"spawner",
						func(ctx context.Context, notifyStart NotifyStartFn) error {
							// we create a value that allows this the spawner to communicate
							// with the subtree in a safe way.
							spawner := newSpawnerClient(ctrlChan)
							return runFn(ctx, notifyStart, spawner)
						},
						opts...,
					),
				}, cleanup, nil
			},
			// if the subtree or the spawner fail, restart both of them
			WithStrategy(OneForAll),
		)

		ctrlChan := make(chan ctrlMsg)
		return supSpec.subtree(dynSubtreeSpec, ctrlChan)
	}
}
