package saboteur

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/cap"
)

// InsertPlan inserts a sabotage plan in this sabotageDB
func (db *sabotageDB) InsertPlan(
	ctx context.Context,
	name planName,
	subtreeName nodeName,
	duration time.Duration,
	period time.Duration,
	attempts uint32,
) error {
	resultChan := make(chan error)
	msg := insertSabotagePlanMsg{
		name:        name,
		subtreeName: subtreeName,
		duration:    duration,
		period:      period,
		attempts:    attempts,
		ResultChan:  resultChan,
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("InsertPlan could not talk to sabotageDB: %w", ctx.Err())
	case db.insertPlanChan <- msg:
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("sabotageDB did not reply back to InsertPlan: %w", ctx.Err())
	case err := <-resultChan:
		return err
	}
}

// RemovePlan removes a sabotage plan from this sabotageDB
func (db *sabotageDB) RemovePlan(
	ctx context.Context,
	name planName,
) error {
	resultChan := make(chan error)
	msg := rmSabotagePlanMsg{
		name:       name,
		ResultChan: resultChan,
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("RemovePlan could not talk to sabotageDB: %w", ctx.Err())
	case db.rmPlanChan <- msg:
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("sabotageDB did not reply back to RemovePlan: %w", ctx.Err())
	case err := <-resultChan:
		return err
	}
}

// StartPlan executes a previously created sabotage plan.
func (db *sabotageDB) StartPlan(
	ctx context.Context,
	name planName,
) error {
	resultChan := make(chan error)
	msg := startSabotagePlanMsg{
		name:       name,
		ResultChan: resultChan,
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("StartPlan could not talk to sabotageDB: %w", ctx.Err())
	case db.startPlanChan <- msg:
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("sabotageDB did not reply back to StartPlan: %w", ctx.Err())
	case err := <-resultChan:
		return err
	}
}

// StopPlan stops the execution of a previously created sabotage plan.
func (db *sabotageDB) StopPlan(
	ctx context.Context,
	name planName,
) error {
	resultChan := make(chan error)
	msg := stopSabotagePlanMsg{
		name:       name,
		ResultChan: resultChan,
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("StopPlan could not talk to sabotageDB: %w", ctx.Err())
	case db.stopPlanChan <- msg:
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("sabotageDB did not reply back to StopPlan: %w", ctx.Err())
	case err := <-resultChan:
		return err
	}
}

// stateLoop is a loop that handles messages that modify the sabotageDB state.
func (db *sabotageDB) stateLoop(ctx context.Context, _ cap.Spawner) error {
	// TODO: cap.Spawner is intended to be used to spawn sabotage plans
	for {
		// TODO: deal with more messages
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-db.insertPlanChan:
			if !ok {
				return errors.New("invalid state: sabotageDB had insertPlanChan closed")
			}
			// Pending
			continue
		case _, ok := <-db.rmPlanChan:
			if !ok {
				return errors.New("invalid state: sabotageDB had rmPlanChan closed")
			}
			// Pending
			continue
		case _, ok := <-db.startPlanChan:
			if !ok {
				return errors.New("invalid state: sabotageDB had startPlanChan closed")
			}
			// Pending
			continue
		case _, ok := <-db.stopPlanChan:
			if !ok {
				return errors.New("invalid state: sabotageDB had stopPlanChan closed")
			}
			// Pending
			continue
		case registerMsg, ok := <-db.registerSignaler:
			// Handle registration message sent by a node that was
			// configured to crash on command.
			if !ok {
				return errors.New("invalid state: sabotageDB had registerSignaler channel closed")
			}

			var saboteur *saboteurNode
			if saboteur, ok = db.saboteurs[registerMsg.SubtreeName]; !ok {
				// Create new saboteur node
				saboteur = &saboteurNode{
					startCount: 0,
					signaler:   make(errSignaler),
				}
				// store in saboteur DB
				db.saboteurs[registerMsg.SubtreeName] = saboteur
			}

			saboteur.startCount++
			select {
			case <-ctx.Done():
				return nil
			case registerMsg.ResultChan <- saboteur.signaler:
			}
		}
	}
}
