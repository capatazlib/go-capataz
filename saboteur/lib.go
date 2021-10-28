package saboteur

import (
	"time"
)

// planName is a human-readable name provided by API users to identify sabotage
// plans.
type planName = string

// nodeName is the runtime name of a subtree that the API tries to sabotage.
type nodeName = string

// stopPlanFn is a function is used to stop the execution of a sabotage plan.
type stopPlanFn = func() error

// errSignaler is a channel used by sabotageDB to send signals to saboteur nodes.
type errSignaler = chan error

// sabotagePlan has the specification of a sabotage execution. This indicates
// how many times, how often and for how long a subtree should receive errors.
type sabotagePlan struct {
	// name of the plan for manipulation from CLI
	name planName
	// name of the subtree
	subtreeName nodeName
	// Time the sabotage plan will last (if 0, indefinitely)
	duration time.Duration
	// Duration between sabotage attempts (defaults to every minute)
	period time.Duration
	// Number of max attempts (if negative, infinitely)
	maxAttempts int32
	// node were sabotage is going to be sent
	node *saboteurNode
}

// saboteurNode is metadata entry of a running capataz subtree.
type saboteurNode struct {
	// number of times the node has registered a start
	startCount uint32
	// channel used to signal errors from sabotageDB to a worker node.
	signaler errSignaler
}

// registerSaboteurMsg is used by saboteur-worker nodes to signal sabotageDB
// that it exists.
type registerSaboteurMsg struct {
	SubtreeName nodeName
	ResultChan  chan errSignaler
}

// listSaboteurNodes lists all the discovered nodes that sabotageDB has
// discovered.
type listSaboteurNodes struct {
	ResultChan chan (chan []nodeName)
}

// listSabotagePlansMsg lists all the plans that have been defined
type listSabotagePlansMsg struct {
	ResultChan chan []sabotagePlan
}

// insertSabotagePlanMsg adds a sabotage plan to sabotageDB.
type insertSabotagePlanMsg struct {
	name        planName
	subtreeName nodeName
	duration    time.Duration
	period      time.Duration
	attempts    uint32
	ResultChan  chan error
}

// rmSabotagePlanMsg removes a sabotage plan from sabotageDB.
type rmSabotagePlanMsg struct {
	name       planName
	ResultChan chan error
}

// startSabotagePlanMsg spawns a goroutine that executes a known sabotage plan.
type startSabotagePlanMsg struct {
	name       planName
	ResultChan chan error
}

// stopSabotagePlanMsg terminates a goroutine that is executing a sabotage plan.
type stopSabotagePlanMsg struct {
	name       planName
	ResultChan chan error
}

// sabotageDB is the record that contains all the sabotage plans we want to
// execute, it also responsible of discovering all the nodes that can be
// sabotaged.
type sabotageDB struct {
	// Channel used to register saboteur workers in the sabotageDB
	registerSignaler chan registerSaboteurMsg
	listPlansChan    chan listSabotagePlansMsg
	insertPlanChan   chan insertSabotagePlanMsg
	rmPlanChan       chan rmSabotagePlanMsg
	startPlanChan    chan startSabotagePlanMsg
	stopPlanChan     chan stopSabotagePlanMsg

	// Collection of known saboteur nodes
	saboteurs    map[nodeName]*saboteurNode
	plans        map[planName]*sabotagePlan
	runningPlans map[planName]stopPlanFn
}

// Server is a HTTP server that allows us to interact to sabotageDB
type Server struct {
	db *sabotageDB
}
