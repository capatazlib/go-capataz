package saboteur

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capatazlib/go-capataz/cap"
)

func (sp *sabotagePlan) toNode() cap.Node {
	return cap.NewWorker(sp.name, sp.stateLoop)
}

func (sp *sabotagePlan) stateLoop(ctx context.Context) error {
	// Create channel that triggers when the plan should stop
	var sabotageTerminationChan <-chan time.Time
	if sp.duration == 0 {
		// When duration is eq to 0, it is a special condition that
		// indicates that the sabotage loop should never stop
		sabotageTerminationChan = make(chan time.Time)
	} else {
		// Ticker that triggers when the sabotage plan should stop
		// altogether
		stopTck := time.NewTicker(sp.duration)
		defer stopTck.Stop()
		sabotageTerminationChan = stopTck.C
	}

	// Ticker that triggers when a new error should be sent
	tck := time.NewTicker(sp.period)
	defer tck.Stop()

	// Keep track of error counts to compare with max attempts later
	// Use 1-indexed counter for easier to understand UI
	errCount := 1

	for {
		// Check if the plan has surpassed the maximum number of attempts
		// Use >= because we are 1 indexed
		if int32(errCount) >= int32(sp.maxAttempts) {
			return nil
		}

		select {
		case <-ctx.Done():
			// Return when the supervision tree is dying
			return nil
		case <-sabotageTerminationChan:
			// Return when the duration of the sabotage plan is done
			return nil
		case <-tck.C:
			// Trigger an error on target node
			err := fmt.Errorf("sabotage plan execution (%d out of %d)", errCount, sp.maxAttempts)
			select {
			case <-ctx.Done():
				return errors.New("terminated while waiting on sabotage signal")
			case sp.node.signaler <- err:
			}
			errCount++
		}
	}
}
