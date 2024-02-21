package server

import (
	shlog "github.com/ethereum-optimism/optimism/shutter-node/log"
	"github.com/ethereum/go-ethereum/log"
	"github.com/hashicorp/go-multierror"
	googrpc "google.golang.org/grpc"
)

type (
	Option  func(*options) error
	options struct {
		listenNetwork string
		listenAddress string
		log           log.Logger
		googopts      []googrpc.ServerOption
	}
)

func (o *options) init() {
	o.googopts = []googrpc.ServerOption{}
	o.log = &shlog.NoopLogger{}
}

func (o *options) apply(opts []Option) error {
	var err error
	for _, opt := range opts {
		err = opt(o)
		if err != nil {
			err = multierror.Append(err)
		}
	}
	return err
}

func WithLogger(log log.Logger) Option {
	return func(o *options) error {
		o.log = log
		return nil
	}
}

func WithListenAddress(network, address string) Option {
	return func(o *options) error {
		o.listenNetwork = network
		o.listenAddress = address
		return nil
	}
}

func WithGRPCOption(opt googrpc.ServerOption) Option {
	return func(o *options) error {
		o.googopts = append(o.googopts, opt)
		return nil
	}
}
