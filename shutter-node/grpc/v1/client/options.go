package client

import (
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/hashicorp/go-multierror"
	googrpc "google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

type (
	Option  func(*options) error
	options struct {
		serverAddress string
		log           log.Logger
		googopts      []googrpc.DialOption
	}
)

var DefaultConnectParams = googrpc.ConnectParams{
	Backoff: backoff.Config{
		BaseDelay:  1 * time.Second,
		Multiplier: 1.6,
		Jitter:     0.2,
		MaxDelay:   120 * time.Second,
	},
	MinConnectTimeout: 20 * time.Second,
}

func (o *options) init() {
	o.googopts = []googrpc.DialOption{
		googrpc.WithConnectParams(DefaultConnectParams),
		googrpc.WithTransportCredentials(insecure.NewCredentials()),
	}
}

func (o *options) apply(opts []Option) error {
	var err error
	for _, opt := range opts {
		err = opt(o)
		if err != nil {
			err = multierror.Append(err)
		}
	}
	if o.log == nil {
		o.log = log.New()
	}
	return err
}

func WithServerAddress(address string) Option {
	return func(o *options) error {
		o.serverAddress = address
		return nil
	}
}

func WithLogger(log log.Logger) Option {
	return func(o *options) error {
		o.log = log
		return nil
	}
}

func WithGRPCOption(opt googrpc.DialOption) Option {
	return func(o *options) error {
		o.googopts = append(o.googopts, opt)
		return nil
	}
}
