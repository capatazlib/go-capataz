package capataz

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Routine interface {
	Go(context.Context) error
}

type RoutineFunc func(context.Context) error

func (f RoutineFunc) Go(ctx context.Context) error {
	return f(ctx)
}

type RoutineFactory func() (Routine, error)

type RestartType uint8

const (
	Permanent RestartType = iota
	Transient
	Temporary
)

type RoutineSpec struct {
	ID             string
	RoutineFactory RoutineFactory
	Restart        RestartType
	// Type
	// Shutdown
}

type RoutineSpecOption func(rs *RoutineSpec)

func WithRestartType(restart RestartType) RoutineSpecOption {
	return func(rs *RoutineSpec) {
		rs.Restart = restart
	}
}

func MakeRoutineSpec(id string, routineFactory RoutineFactory, options ...RoutineSpecOption) RoutineSpec {
	rs := &RoutineSpec{}
	for _, opt := range options {
		opt(rs)
	}
	rs.ID = id
	rs.RoutineFactory = routineFactory
	return *rs
}

func MakeRoutineSpecFunc(id string, f RoutineFunc, options ...RoutineSpecOption) RoutineSpec {
	return MakeRoutineSpec(id, func() (Routine, error) {
		return f, nil
	}, options...)
}

type exitReason struct {
	reason interface{}
}

type exitReasonValue uint8

const (
	exitNone exitReasonValue = iota
	exitShutdown
	exitNormal
)

func (r exitReason) isNone() bool {
	v, ok := r.reason.(exitReasonValue)
	return ok && v == exitNone
}

func (r exitReason) isNormal() bool {
	v, ok := r.reason.(exitReasonValue)
	return ok && v == exitNormal
}

func (r exitReason) isShutdown() bool {
	v, ok := r.reason.(exitReasonValue)
	return ok && v == exitShutdown
}

func (r exitReason) isError() bool {
	_, ok := r.reason.(error)
	return ok
}

type routineSupervisor struct {
	sync.Mutex
	id                  string
	uuid                uuid.UUID
	routine             Routine
	stoppedBySupervisor bool
	done                chan struct{}
	exitReason          exitReason
	cancelContext       context.CancelFunc
	wg                  sync.WaitGroup
}

func newRoutineSupervisor(spec RoutineSpec) (*routineSupervisor, error) {
	routine, err := spec.RoutineFactory()

	if err != nil {
		return nil, fmt.Errorf("[%s] routine could not start", spec.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	rs := &routineSupervisor{
		id:            spec.ID,
		uuid:          uuid.New(),
		routine:       routine,
		done:          make(chan struct{}, 1),
		exitReason:    exitReason{exitNone},
		cancelContext: cancel,
	}

	rs.wg.Add(1)

	go func() {
		err := rs.routine.Go(ctx)
		rs.Lock()
		defer rs.Unlock()

		if err == ctx.Err() {
			rs.exitReason = exitReason{exitShutdown}
		} else if err != nil {
			rs.exitReason = exitReason{err}
		} else {
			rs.exitReason = exitReason{exitNormal}
		}

		rs.wg.Done()
		rs.done <- struct{}{}
		close(rs.done)
	}()
	return rs, nil
}

func (rs *routineSupervisor) stop() error {
	rs.Lock()
	if !rs.exitReason.isNone() {
		rs.Unlock()
		return fmt.Errorf("[%s]-[%s] routine already stopped", rs.id, rs.uuid)
	}

	rs.stoppedBySupervisor = true
	rs.cancelContext()
	rs.Unlock()

	rs.wg.Wait()
	return nil
}
