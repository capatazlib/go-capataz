package saboteur

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/cap"
)

// ListNodes inserts a sabotage node in this sabotageDB
func (db *sabotageDB) ListNodes(
	ctx context.Context,
) ([]string, error) {
	resultChan := make(chan []nodeName, 1)
	defer close(resultChan)

	msg := listSaboteurNodesMsg{
		ResultChan: resultChan,
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ListNodes could not talk to sabotageDB: %w", ctx.Err())
	case db.listNodesChan <- msg:
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("sabotageDB did not reply back to ListNodes: %w", ctx.Err())
	case nodes := <-resultChan:
		return nodes, nil
	}
}

// ListPlans inserts a sabotage plan in this sabotageDB
func (db *sabotageDB) ListPlans(
	ctx context.Context,
) ([]sabotagePlanWithRunningStatus, error) {
	resultChan := make(chan []sabotagePlanWithRunningStatus, 1)
	defer close(resultChan)

	msg := listSabotagePlansMsg{
		ResultChan: resultChan,
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ListPlans could not talk to sabotageDB: %w", ctx.Err())
	case db.listPlansChan <- msg:
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("sabotageDB did not reply back to ListPlans: %w", ctx.Err())
	case plans := <-resultChan:
		return plans, nil
	}
}

// InsertPlan inserts a sabotage plan in this sabotageDB
func (db *sabotageDB) InsertPlan(
	ctx context.Context,
	name planName,
	subtreeName nodeName,
	duration time.Duration,
	period time.Duration,
	attempts uint32,
) error {
	resultChan := make(chan error, 1)
	defer close(resultChan)

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

func (msg *insertSabotagePlanMsg) toPlan(node *saboteurNode) *sabotagePlan {
	return &sabotagePlan{
		name:        msg.name,
		subtreeName: msg.subtreeName,
		duration:    msg.duration,
		period:      msg.period,
		maxAttempts: int32(msg.attempts),
		node:        node,
	}
}

// RemovePlan removes a sabotage plan from this sabotageDB
func (db *sabotageDB) RemovePlan(
	ctx context.Context,
	name planName,
) error {
	resultChan := make(chan error, 1)
	defer close(resultChan)

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
	resultChan := make(chan error, 1)
	defer close(resultChan)

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
	resultChan := make(chan error, 1)
	defer close(resultChan)

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
func (db *sabotageDB) stateLoop(ctx context.Context, spawner cap.Spawner) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		case msg, ok := <-db.listNodesChan:
			// Check invalid state
			if !ok {
				return errors.New("invalid state: sabotageDB had listNodesChan closed")
			}

			nodes := make([]string, 0, len(db.saboteurs))
			for n := range db.saboteurs {
				nodes = append(nodes, n)
			}
			msg.ResultChan <- nodes

		case msg, ok := <-db.listPlansChan:
			// Check invalid state
			if !ok {
				return errors.New("invalid state: sabotageDB had listPlansChan closed")
			}
			plans := make([]sabotagePlanWithRunningStatus, 0, len(db.plans))
			for _, p := range db.plans {
				_, running := db.runningPlans[p.name]
				plans = append(plans, sabotagePlanWithRunningStatus{
					sabotagePlan: *p,
					running:      running,
				})
			}
			msg.ResultChan <- plans

		case msg, ok := <-db.insertPlanChan:
			// Check invalid state
			if !ok {
				return errors.New("invalid state: sabotageDB had insertPlanChan closed")
			}

			// Check given subtree name exists
			node, ok := db.saboteurs[msg.subtreeName]
			if !ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on InsertPlan result ack")
				case msg.ResultChan <- errors.New("invalid node name: not found"):
				}
				continue
			}

			// Check plan is not already present
			if _, ok := db.plans[msg.name]; ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on InsertPlan result ack")
				case msg.ResultChan <- errors.New("plan name already registered"):
				}
				continue
			}

			// Insert plan
			plan := msg.toPlan(node)
			db.plans[msg.name] = plan

			// Notify back success
			select {
			case <-ctx.Done():
				return errors.New("terminated while waiting on InsertPlan result ack")
			case msg.ResultChan <- nil:
			}

		case msg, ok := <-db.rmPlanChan:
			// Check invalid state
			if !ok {
				return errors.New("invalid state: sabotageDB had rmPlanChan closed")
			}

			// Check plan is present
			_, ok = db.plans[msg.name]
			if !ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on RemovePlan result ack")
				case msg.ResultChan <- errors.New("invalid plan name: not found"):
				}
				continue
			}

			// Check plan is running so that we stop it and remove it from running plans
			if stopPlan, ok := db.runningPlans[msg.name]; ok {
				err := stopPlan()
				if err != nil {
					select {
					case <-ctx.Done():
						return errors.New("terminated while waiting on RemovePlan result ack")
					case msg.ResultChan <- fmt.Errorf("plan could not be stopped: %w", err):
					}
					continue
				}
				delete(db.runningPlans, msg.name)
			}

			// Remove plan from plan specification map
			delete(db.plans, msg.name)

			// Notify back success
			select {
			case <-ctx.Done():
				return errors.New("terminated while waiting on InsertPlan result ack")
			case msg.ResultChan <- nil:
			}

		case msg, ok := <-db.startPlanChan:
			// Check invalid state
			if !ok {
				return errors.New("invalid state: sabotageDB had startPlanChan closed")
			}

			// Check plan is present
			plan, ok := db.plans[msg.name]
			if !ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on StartPlan result ack")
				case msg.ResultChan <- errors.New("invalid plan name: not found"):
				}
				continue
			}

			// Check plan is not already running
			if _, ok := db.runningPlans[msg.name]; ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on StartPlan result ack")
				case msg.ResultChan <- errors.New("plan already running"):
				}
				continue
			}

			// Spawn the plan worker node
			stopPlanFn, err := spawner.Spawn(plan.toNode())
			if err != nil {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on StartPlan result ack")
				case msg.ResultChan <- fmt.Errorf("could not start plan: %w", err):
				}
				continue
			}

			// Insert to runningPlans state
			db.runningPlans[msg.name] = stopPlanFn

			// Notify back success
			select {
			case <-ctx.Done():
				return errors.New("terminated while waiting on StartPlan result ack")
			case msg.ResultChan <- nil:
			}

		case msg, ok := <-db.stopPlanChan:
			// Check invalid state
			if !ok {
				return errors.New("invalid state: sabotageDB had stopPlanChan closed")
			}
			// Check plan is present
			_, ok = db.plans[msg.name]
			if !ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on StopPlan result ack")
				case msg.ResultChan <- errors.New("invalid plan name: not found"):
				}
				continue
			}

			// Check plan is running so that we stop it and remove it from running plans
			stopPlan, ok := db.runningPlans[msg.name]

			if !ok {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on StopPlan result ack")
				case msg.ResultChan <- fmt.Errorf("plan is not running"):
				}
			}

			err := stopPlan()
			// Check stop plan did not fail
			if err != nil {
				select {
				case <-ctx.Done():
					return errors.New("terminated while waiting on StopPlan result ack")
				case msg.ResultChan <- fmt.Errorf("plan could not be stopped: %w", err):
				}
				continue
			}

			// Remove plan from running state
			delete(db.runningPlans, msg.name)

			// Notify back success
			select {
			case <-ctx.Done():
				return errors.New("terminated while waiting on StopPlan result ack")
			case msg.ResultChan <- nil:
			}

		case registerMsg, ok := <-db.registerSignaler:
			if !ok {
				return errors.New("invalid state: sabotageDB had registerSignaler channel closed")
			}

			var saboteur *saboteurNode
			if saboteur, ok = db.saboteurs[registerMsg.SubtreeName]; !ok {
				// Create saboteur node for the first time
				saboteur = &saboteurNode{
					startCount: 0,
					signaler:   make(errSignaler),
				}
				// store in saboteur DB
				db.saboteurs[registerMsg.SubtreeName] = saboteur
			}

			// Increase saboteur node start count
			saboteur.startCount++
			select {
			case <-ctx.Done():
				return errors.New("terminated while waiting on saboteur registration")
			case registerMsg.ResultChan <- saboteur.signaler:
			}
		}
	}
}
