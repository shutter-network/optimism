package client

import (
	"context"

	grpc "github.com/ethereum-optimism/optimism/shutter-node/grpc/v1"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	googrpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type Client struct {
	options *options

	log    log.Logger
	conn   *googrpc.ClientConn
	client grpc.DecryptionKeyServiceClient
}

func NewClient(opts ...Option) (*Client, error) {
	o := &options{}
	o.init()
	err := o.apply(opts)
	if err != nil {
		return nil, errors.Wrap(err, "apply options")
	}
	return &Client{
		options: o,
		log:     o.log,
	}, nil
}

func (c *Client) Init(ctx context.Context) error {
	conn, err := googrpc.DialContext(ctx, c.options.serverAddress, c.options.googopts...)
	if err != nil {
		return errors.Wrap(err, "dial server")
	}
	c.conn = conn
	c.client = grpc.NewDecryptionKeyServiceClient(conn)
	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) waitState(ctx context.Context) bool {
	for {
		retry := true
		state := c.conn.GetState()
		switch state {
		case connectivity.Ready:
			retry = false
		case connectivity.Connecting:
		case connectivity.TransientFailure:
		// NOTE: if the configured gRPC server
		// address is incorrect, we will stay in this
		// state forever
		case connectivity.Idle:
			c.conn.Connect()
		case connectivity.Shutdown:
			c.log.Info("gRPC connection shutting down")
			return false
		default:
		}
		if retry {
			c.log.Info("gRPC connection not ready, waiting", "state", state.String())
			ok := c.conn.WaitForStateChange(ctx, state)
			// this can only mean the passed in ctx is done
			if !ok {
				return false
			}
			continue
		}
		c.log.Info("gRPC connection READY")
		return true
	}
}

// Returns a promise
func (c *Client) GetKey(ctx context.Context, block uint) <-chan *DecryptionKeyResult {
	key := make(chan *DecryptionKeyResult, 1)
	req := &grpc.GetDecryptionKeyRequest{
		Block: uint64(block),
	}
	opts := []googrpc.CallOption{}

	go func(ctx context.Context, keyChan chan<- *DecryptionKeyResult) {
		defer close(keyChan)
		ok := c.waitState(ctx)
		if !ok {
			// ctx done, or rpc shutting down
			return
		}

		resp, err := c.client.GetDecryptionKey(ctx, req, opts...)
		// XXX: although we waited for a state, this can still fail again
		// due to the state. We don't currently catch this and try again..
		// --> is this transparently retrying?
		decrKey := ToResult(resp.GetDecryptionKey(), err)
		// promise, so shouldn't be blocking unlesss we got
		// a send somewhere else
		select {
		case <-ctx.Done():
		case keyChan <- &decrKey:
		}
	}(ctx, key)

	return key
}
