package capataz

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type routineInfo struct {
	spec RoutineSpec
	rs   *routineSupervisor
}

type SupervisorSpec struct {
	MaxRestarts      uint
	MaxRestartWindow time.Duration
	RoutineSpecs     []RoutineSpec
	RestartStrategy  RestartStrategy
	Restart          Restart
	StartOrder       Order
	StopOrder        Order
}

type SupervisorSpecOption func(ss *SupervisorSpec)

func WithMaxRestarts(n uint) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.MaxRestarts = n
	}
}

func WithMaxRestartWindow(d time.Duration) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.MaxRestartWindow = d
	}
}

func WithRoutineSpecs(specs []RoutineSpec) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.RoutineSpecs = specs
	}
}

func WithRestartStrategy(strategy RestartStrategy) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.RestartStrategy = strategy
	}
}

func WithRestart(restart Restart) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.Restart = restart
	}
}

func WithStartOrder(order Order) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.StartOrder = order
	}
}

func WithStopOrder(order Order) SupervisorSpecOption {
	return func(ss *SupervisorSpec) {
		ss.StopOrder = order
	}
}

type CountResult struct {
	Specs       int
	Active      int
	Supervisors int
	Workers     int
}

type Supervisor interface {
	// CountRoutines() CountResult
	// DeleteChild(id string) error
	// GetChildSpec(id string) (ChildSpec, error)
	// RestartChild(id string) error
	// StartChild(childSpec ChildSpec) error
	// TerminateChild(id string) error
}

type supervisor struct {
	sync.RWMutex
	spec              SupervisorSpec
	routineSpecs      []RoutineSpec
	routines          map[string]*routineInfo
	cancelContext     context.CancelFunc
	restartTracker    restartTracker
	wg                sync.WaitGroup
	monitoringStarted bool
}

func supervisorFactory(spec SupervisorSpec) RoutineFactory {
	return func() (Routine, error) {
		return newSupervisor(spec)
	}
}

func newSupervisor(spec SupervisorSpec) (*supervisor, error) {
	s := &supervisor{
		spec:         spec,
		routineSpecs: cloneRoutineSpecs(spec.RoutineSpecs),
		routines:     make(map[string]*routineInfo),
		restartTracker: restartTracker{
			max:    spec.MaxRestarts,
			window: spec.MaxRestartWindow,
		},
	}

	s.spec.StartOrder(s.routineSpecs, func(spec RoutineSpec) error {
		return s.startRoutine(spec)
	})

	return s, nil
}

func (s *supervisor) Go(ctx context.Context) error {
	if s.monitoringStarted {
		return nil
	}

	return s.startMonitoring(ctx)
}

func (s *supervisor) startMonitoring(ctx context.Context) error {
	s.monitoringStarted = true
	s.wg.Add(1)
	ctx, cancel := context.WithCancel(ctx)
	s.cancelContext = cancel

	<-ctx.Done()

	s.stopRoutines()
	s.wg.Done()

	return ctx.Err()
}

func (s *supervisor) onRoutineTerminated(rs *routineSupervisor, spec RoutineSpec) {
	isTemp := spec.Restart == Temporary
	isExpected := rs.stoppedBySupervisor && rs.exitReason.isShutdown()
	isTransientOk := spec.Restart == Transient && !rs.exitReason.isError()
	now := time.Now()

	if isTemp || isExpected || isTransientOk {
		return
	}

	specs := s.spec.RestartStrategy(s, spec)

	if len(specs) > 0 && s.restartTracker.reachedMaxInWindow(now) {
		// SHUTDOWN
		s.cancelContext()
		return
	}

	s.restartTracker.addRestart(now)
	s.spec.Restart(s, specs)
}

func (s *supervisor) Stop() {
	s.cancelContext()
	s.wg.Wait()
}

func (s *supervisor) StopRoutine(id string) error {
	s.RLock()
	defer s.RUnlock()
	return s.stopRoutine(id)
}

func (s *supervisor) stopRoutine(id string) error {
	ri, ok := s.routines[id]

	if !ok {
		return fmt.Errorf("[%s] cannot stop routine: not found", id)
	}

	return ri.rs.stop()
}

func (s *supervisor) stopRoutines() error {
	return s.spec.StopOrder(s.routineSpecs, func(spec RoutineSpec) error {
		return s.stopRoutine(spec.ID)
	})
}

func (s *supervisor) startRoutine(spec RoutineSpec) error {
	rs, err := newRoutineSupervisor(spec)

	if err != nil {
		return err
	}

	s.routines[spec.ID] = &routineInfo{
		spec: spec,
		rs:   rs,
	}

	go func() {
		<-rs.done
		s.onRoutineTerminated(rs, spec)
	}()

	return nil
}

type Order func(specs []RoutineSpec, fn func(spec RoutineSpec) error) error

func LeftToRight(specs []RoutineSpec, fn func(spec RoutineSpec) error) error {
	for _, spec := range specs {
		if err := fn(spec); err != nil {
			return err
		}
	}
	return nil
}

func RightToLeft(specs []RoutineSpec, fn func(spec RoutineSpec) error) error {
	return LeftToRight(reverseRoutineSpecs((specs)), fn)
}

type RestartStrategy func(*supervisor, RoutineSpec) []RoutineSpec

func OneForOne(s *supervisor, failedSpec RoutineSpec) []RoutineSpec {
	return []RoutineSpec{failedSpec}
}

func OneForAll(s *supervisor, failedSpec RoutineSpec) []RoutineSpec {
	return cloneRoutineSpecs(s.routineSpecs)
}

func RestForOne(s *supervisor, failedSpec RoutineSpec) []RoutineSpec {
	restarts := make(map[string]bool)
	found := false

	s.spec.StartOrder(s.routineSpecs, func(spec RoutineSpec) error {
		found = found || spec.ID == failedSpec.ID
		if found {
			restarts[spec.ID] = true
		}
		return nil
	})

	specs := make([]RoutineSpec, len(restarts))
	for _, spec := range s.routineSpecs {
		if restarts[spec.ID] {
			specs = append(specs, spec)
		}
	}

	return specs
}

type Restart func(s *supervisor, specs []RoutineSpec) error

func RestartEach(order Order) Restart {
	return func(s *supervisor, specs []RoutineSpec) error {
		return order(specs, func(spec RoutineSpec) error {
			s.stopRoutine(spec.ID)
			return s.startRoutine(spec)
		})
	}
}

func RestartInOrder(order Order) Restart {
	return func(s *supervisor, specs []RoutineSpec) error {
		order(specs, func(spec RoutineSpec) error {
			return s.stopRoutine(spec.ID)
		})

		return order(specs, func(spec RoutineSpec) error {
			return s.startRoutine(spec)
		})
	}
}

func RestartRevOrder(order Order) Restart {
	return func(s *supervisor, specs []RoutineSpec) error {
		order(reverseRoutineSpecs(specs), func(spec RoutineSpec) error {
			return s.stopRoutine(spec.ID)
		})

		return order(specs, func(spec RoutineSpec) error {
			return s.startRoutine(spec)
		})
	}
}

func cloneRoutineSpecs(specs []RoutineSpec) []RoutineSpec {
	cp := make([]RoutineSpec, len(specs))
	copy(cp, specs)
	return cp
}

func reverseRoutineSpecs(specs []RoutineSpec) []RoutineSpec {
	size := len(specs)
	cp := make([]RoutineSpec, size)

	for i, spec := range specs {
		cp[size-i-1] = spec
	}
	return cp
}

type restartTracker struct {
	max    uint
	window time.Duration
	s      []time.Time
}

func (rt restartTracker) reachedMaxInWindow(t time.Time) bool {
	if rt.s == nil || len(rt.s) == 0 {
		return false
	}

	w := t.Add(-rt.window)
	c := uint(0)

	for _, v := range rt.s {
		if v.Equal(w) || v.After(w) {
			c++
			continue
		}
		break
	}

	return c == rt.max
}

func (rt restartTracker) addRestart(t time.Time) {
	if rt.s == nil {
		rt.s = make([]time.Time, 0, rt.max)
	}
	rt.s = append([]time.Time{t}, rt.s[0:rt.max-1]...)
}
