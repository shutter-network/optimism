package shutter_test

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type (
	CheckFunction func(db *gorm.DB, ev *TestEvent) error
	WaitFunction  func(ctx context.Context, db *gorm.DB) error
)

type Option func(*options) error

func WithPreCheck(fn CheckFunction) Option {
	return func(o *options) error {
		o.preChecks = append(o.preChecks, fn)
		return nil
	}
}

func WithPostCheck(fn CheckFunction) Option {
	return func(o *options) error {
		o.postChecks = append(o.postChecks, fn)
		return nil
	}
}

func WithFinalCheck(fn CheckFunction) Option {
	return func(o *options) error {
		o.finalChecks = append(o.finalChecks, fn)
		return nil
	}
}

type options struct {
	preChecks   []CheckFunction
	postChecks  []CheckFunction
	finalChecks []CheckFunction
}

func (o *options) applyDefaults() {
	o.finalChecks = []CheckFunction{}
	o.postChecks = []CheckFunction{}
	o.preChecks = []CheckFunction{}
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

func NewTestEvent(name string, value any, opts ...Option) *TestEvent {
	o := &options{}
	o.applyDefaults()
	err := o.apply(opts)
	if err != nil {
		panic(err)
	}
	return &TestEvent{
		Value:      value,
		Name:       name,
		resultChan: make(chan *Result, 1),
		opts:       o,
	}
}

type final struct{}

func (_ final) String() string {
	return "Close"
}

func Close() *TestEvent {
	return NewTestEvent("Close", final{})
}

type Result struct {
	Value any
	Error error
}

type TestEvent struct {
	opts *options

	Value      any
	Name       string
	resultChan chan *Result
}

func (te *TestEvent) String() string {
	return te.Name
}

func callFns(db *gorm.DB, te *TestEvent, fns []CheckFunction) error {
	var multiErr error
	if fns == nil {
		return nil
	}
	for _, fn := range fns {
		if err := fn(db, te); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	if multiErr != nil {
		return multiErr
	}
	return nil
}

func (te *TestEvent) PreCheck(db *gorm.DB) error {
	return callFns(db, te, te.opts.preChecks)
}

func (te *TestEvent) PostCheck(db *gorm.DB) error {
	return callFns(db, te, te.opts.postChecks)
}

func (te *TestEvent) FinalCheck(db *gorm.DB) error {
	return callFns(db, te, te.opts.finalChecks)
}

// SetResult sets the value and error for the result of the testevent.
// If this is called, SetResult should not be called anymore.
func (te *TestEvent) SetResult(ctx context.Context, value any, err error) error {
	select {
	case te.resultChan <- &Result{
		Value: value,
		Error: err,
	}:
		defer close(te.resultChan)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (te *TestEvent) WaitResult(ctx context.Context) (any, error) {
	select {
	case result, ok := <-te.resultChan:
		if !ok {
			return nil, errors.New("result channel closed")
		}
		return result.Value, result.Error
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "context cancel while waiting for result")
	}
}
