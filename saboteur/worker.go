package saboteur

import (
	"context"
	"strings"

	"github.com/capatazlib/go-capataz/cap"
)

func (db *sabotageDB) registerNode(ctx context.Context, subtreeName string) errSignaler {
	errSignalerChan := make(chan errSignaler)
	msg := registerSaboteurMsg{
		SubtreeName: subtreeName,
		ResultChan:  errSignalerChan,
	}

	// Send new node registration to sabotageDB
	select {
	case <-ctx.Done():
		return nil
	case db.registerSignaler <- msg:
	}

	// Wait for result from sabotageDB
	select {
	case <-ctx.Done():
		return nil
	case signaler := <-errSignalerChan:
		return signaler
	}
}

func (db *sabotageDB) GenWorker() cap.Node {
	return cap.NewWorkerWithNotifyStart(
		"saboteur-worker",
		func(ctx context.Context, notifyStart cap.NotifyStartFn) error {
			workerName, ok := cap.GetWorkerName(ctx)
			// When not having a context that has a worker name, we
			// wait for termination. This branch is unlikely to
			// happen.
			if !ok {
				<-ctx.Done()
				return nil
			}

			// Gather name of current subtree
			subtreeIx := strings.LastIndex(workerName, "/")
			subtreeName := workerName[:subtreeIx]

			// Register and get sabetour signaler from database
			errSignaler := db.registerNode(ctx, subtreeName)

			// returns nil when context.Context is done
			if errSignaler == nil {
				return nil
			}

			// Register the worker has started after registering it
			// to sabotageDB.
			notifyStart(nil)

			select {
			case <-ctx.Done():
				return nil
			case err := <-errSignaler:
				// Wait for sabotage signal and return error
				return err
			}
		})
}
