package saboteur

import (
	"context"

	"github.com/capatazlib/go-capataz/cap"
)

func (sp *sabotagePlan) toNode() cap.Node {
	return cap.NewWorker(sp.name, sp.stateLoop)
}

func (sp *sabotagePlan) stateLoop(ctx context.Context) error {
	// Implementation Pending
	<-ctx.Done()
	return nil
}
