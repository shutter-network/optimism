package manager_test

import (
	"context"

	"gorm.io/gorm"
)

type (
	CheckFunction func(db *gorm.DB, ev *TestEvent) error
	WaitFunction  func(ctx context.Context, db *gorm.DB) error
)

type Option func(*options) error

func WithPreCheck(fn CheckFunction) Option {
	return func(o *options) error {
		o.preCheck = fn
		return nil
	}
}

func WithPostCheck(fn CheckFunction) Option {
	return func(o *options) error {
		o.postCheck = fn
		return nil
	}
}

func WithFinalCheck(fn CheckFunction) Option {
	return func(o *options) error {
		o.finalCheck = fn
		return nil
	}
}

func WithWait(fn WaitFunction) Option {
	return func(o *options) error {
		o.wait = fn
		return nil
	}
}

type options struct {
	preCheck   CheckFunction
	wait       WaitFunction
	postCheck  CheckFunction
	finalCheck CheckFunction
}

func (o *options) applyDefaults() {
	o.finalCheck = nil
	o.postCheck = nil
	o.preCheck = nil
	o.wait = nil
}

func (o *options) apply(opts []Option) error {
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewTestEvent(value any, opts ...Option) *TestEvent {
	o := &options{}
	o.applyDefaults()
	err := o.apply(opts)
	if err != nil {
		panic(err)
	}
	return &TestEvent{
		Value:  value,
		Result: make(chan any, 1),
		Error:  make(chan error, 1),
		opts:   o,
	}
}

type TestEvent struct {
	Value any

	Result chan any
	Error  chan error

	opts *options
}

func (te *TestEvent) PreCheck(db *gorm.DB) error {
	fn := te.opts.preCheck
	if fn == nil {
		return nil
	}
	return fn(db, te)
}

func (te *TestEvent) PostCheck(db *gorm.DB) error {
	fn := te.opts.postCheck
	if fn == nil {
		return nil
	}
	return fn(db, te)
}

func (te *TestEvent) FinalCheck(db *gorm.DB) error {
	fn := te.opts.finalCheck
	if fn == nil {
		return nil
	}
	return fn(db, te)
}

func (te *TestEvent) Wait(ctx context.Context, db *gorm.DB) error {
	fn := te.opts.wait
	if fn == nil {
		return nil
	}
	return fn(ctx, db)
}

// SetError sets an error for the result of the testevent.
// If this is called, SetResult should not be called anymore.
func (te *TestEvent) SetError(ctx context.Context, value error) {
	defer te.cleanup()
	select {
	case te.Error <- value:
		te.Result <- nil
		close(te.Error)
		close(te.Result)
		return
	case <-ctx.Done():
		return
	}
}

func (te *TestEvent) cleanup() {
	close(te.Error)
	close(te.Result)
}

// SetResult sets a value to the result of the testevent and indicates
// successful execution.
// If this is called, SetError should not be called anymore.
func (te *TestEvent) SetResult(ctx context.Context, value any) {
	defer te.cleanup()
	select {
	case te.Result <- value:
		te.Error <- nil
		return
	case <-ctx.Done():
		return
	}
}

func (te *TestEvent) WaitResult(ctx context.Context) (any, error) {
	var (
		err            error
		res            any
		hasErr, hasRes bool
	)
	sig := make(chan struct{}, 2)
	defer close(sig)
	for {
		select {
		case <-sig:
			if hasErr && hasRes {
				return res, err
			}
		case res = <-te.Result:
			hasRes = true
			sig <- struct{}{}
		case err = <-te.Error:
			hasErr = true
			sig <- struct{}{}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
