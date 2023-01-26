package saboteur

import (
	"io/ioutil"
	"net/http"

	"github.com/capatazlib/go-capataz/cap"
	"github.com/sirupsen/logrus"
)

type saboteurSettings struct {
	ll              logrus.FieldLogger
	tlsKeyFilepath  string
	tlsCertFilepath string
}

// Opt provides ways to customize the SaboteurManager API.
//
// @since 0.2.1
type Opt func(*saboteurSettings)

// WithLogger adds a specific `logrus.FieldLogger` to the logging mechanisms of
// the saboteur subtree. It defaults to an empty logger otherwise.
//
// @since 0.2.1
func WithLogger(ll logrus.FieldLogger) Opt {
	return func(opts *saboteurSettings) {
		opts.ll = ll
	}
}

// WithTLS specifies that we want the HTTP server running the saboteur API to
// use a specific TLS configuration.
//
// @since 0.2.1
func WithTLS(certFilepath, keyFilepath string) Opt {
	return func(opts *saboteurSettings) {
		opts.tlsCertFilepath = certFilepath
		opts.tlsKeyFilepath = keyFilepath
	}
}

// NewSaboteurManager creates a subtree that runs workers that trigger sabotage
// plans in nodes that get created using the WorkerGenerator.
//
// HTTP Server Argument
//
// The factory function must receive an http.Server value that:
//
// * Has an Address attribute intialized.
//
// * Does not have a Handler attribute initialized.
//
// Users may modify the given http.Server instance in other ways if they so
// desire.
//
// Options
//
// This function receives various functions that provide logging or TLS support
// to the HTTP server.
//
// @since 0.2.1
func NewSaboteurManager(
	httpServer *http.Server,
	opts ...Opt,
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

	// Use a logger that discards all output by default.
	defaultLogger := logrus.New()
	defaultLogger.SetOutput(ioutil.Discard)

	settings := &saboteurSettings{
		ll:              defaultLogger,
		tlsKeyFilepath:  "",
		tlsCertFilepath: "",
	}

	for _, optFn := range opts {
		optFn(settings)
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

			server := NewServer(settings.ll, db)
			serverNode, err := server.NewHTTPNode(
				httpServer,
				settings.tlsCertFilepath,
				settings.tlsKeyFilepath,
			)

			if err != nil {
				return nil, nil, err
			}

			cleanupFn := func() error {
				// Remove all running plan data on a supervisor
				// restart. These goroutines will be guaranteed
				// to be terminated because of the supervision
				// tree shutdown mechanism.
				db.runningPlans = make(map[planName]stopPlanFn)
				return nil
			}

			return []cap.Node{dbNode, serverNode}, cleanupFn, nil
		},
	)

	return db, spec
}
