package writer

import (
	"context"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/pkg/errors"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

func (w *DBWriter) Start(ctx context.Context, runner service.Runner) error {
	err := w.Init(ctx)
	if err != nil {
		return err
	}
	runner.Defer(w.Cleanup)
	runner.Go(func() error {
		err := w.synchronizedWriteLoop(ctx)
		// close the db session's context
		// cancel(err)
		return err
	})
	if w.client != nil {
		err = runner.StartService(w.client)
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	ErrEventTypeNotSupported = errors.New("event type not supported")
	ErrUnrecoverable         = errors.New("handle-event error unrecoverable")
)

func (w *DBWriter) HandleEventSync(ev any) error {
	w.log.Info("processing event", "event", ev)
	var err error
	switch evTyped := ev.(type) {
	case *syncevent.EonPublicKey:
		err = w.handleEonKey(evTyped)
	case *syncevent.KeyperSet:
		err = w.handleKeyperSet(evTyped)
	case *syncevent.LatestBlock:
		err = w.handleLatestBlock(evTyped)
	case *syncevent.ShutterState:
		err = w.handleShutterActive(evTyped)
	case *models.Epoch:
		err = w.handleNewEpoch(evTyped)
	default:
		return ErrEventTypeNotSupported
	}
	if err != nil {
		// NOTE: for now, all errors are unrecoverable
		// and will cause the errorgroup to shut down
		return errors.Wrap(ErrUnrecoverable, err.Error())
	}
	return nil
}

// ProcessNextEvent will try to consume an event from the
// event channel and process it synchronously.
// CAREFUL, do not call this when simultaneously the DBWriter
// service is running!
func (w *DBWriter) ProcessNextEvent(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ev := <-w.eventChan:
		err := w.HandleEventSync(ev)
		if errors.Is(err, ErrEventTypeNotSupported) {
			w.log.Error("event not supported, skipping event", "error", err)
			return nil
		}
		if errors.Is(err, ErrUnrecoverable) {
			// cause the error group to cancel
			return errors.Unwrap(err)
		}
		if err != nil {
			w.log.Error("got error in write loop, skipping event", "error", err)
			return nil
		}
	}
	return nil
}

func (w *DBWriter) synchronizedWriteLoop(ctx context.Context) error {
	for {
		if err := w.ProcessNextEvent(ctx); err != nil {
			return err
		}
	}
}

func (w *DBWriter) Cleanup() {
}

func (w *DBWriter) Close(err error) {
	w.cancelDB(err)
}
