package saboteur

import "github.com/capatazlib/go-capataz/cap"

// NewSaboteurManager creates a subtree that runs workers that trigger sabotage
// plans in nodes that get created using the WorkerGenerator.
//
// @since 0.2.1
func NewSaboteurManager(
	opts ...cap.Opt,
) (WorkerGenerator, cap.SupervisorSpec) {
	// The state is created outside the supervision tree as we rely on the
	// database to create nodes.
	db := &sabotageDB{
		registerSignaler: make(chan registerSaboteurMsg),
		listNodesChan:    make(chan listSaboteurNodesMsg),
		listPlansChan:    make(chan listSabotagePlansMsg),
		insertPlanChan:   make(chan insertSabotagePlanMsg),
		rmPlanChan:       make(chan rmSabotagePlanMsg),
		startPlanChan:    make(chan startSabotagePlanMsg),
		stopPlanChan:     make(chan stopSabotagePlanMsg),
		saboteurs:        make(map[nodeName]*saboteurNode),
		plans:            make(map[planName]*sabotagePlan),
		runningPlans:     make(map[planName]stopPlanFn),
	}

	spec := cap.NewSupervisorSpec(
		"saboteur",
		func() ([]cap.Node, cap.CleanupResourcesFn, error) {
			// Create subtree responsible of the state management
			// of saboteur registration and plan execution.
			dbNode := cap.NewDynSubtree(
				"sabotageDB",
				db.stateLoop,
				[]cap.Opt{},
			)

			// TODO: create server node here

			cleanupFn := func() error {
				// Remove all running plan data on a supervisor
				// restart. These goroutines will be guaranteed
				// to be terminated because of the supervision
				// tree shutdown mechanism.
				db.runningPlans = make(map[planName]stopPlanFn)
				return nil
			}

			return []cap.Node{dbNode}, cleanupFn, nil
		},
		opts...,
	)

	return db, spec
}
