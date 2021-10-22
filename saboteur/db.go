package saboteur

import (
	"context"
	"errors"

	"github.com/capatazlib/go-capataz/cap"
)

// stateLoop is a loop that handles messages that modify the sabotageDB state.
func (db *sabotageDB) stateLoop(ctx context.Context, _ cap.Spawner) error {
	// TODO: cap.Spawner is intended to be used to spawn sabotage plans
	for {
		// TODO: deal with more messages
		select {
		case <-ctx.Done():
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
