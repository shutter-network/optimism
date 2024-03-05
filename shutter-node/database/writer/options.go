package writer

type options struct {
	unitTesting bool
}

type Option func(*options) error

func defaultOptions() *options {
	return &options{
		unitTesting: false,
	}
}

func (o *options) apply(opts ...Option) error {
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return err
		}
	}
	return nil
}

func UnitTesting() Option {
	return func(o *options) error {
		o.unitTesting = true
		return nil
	}
}
