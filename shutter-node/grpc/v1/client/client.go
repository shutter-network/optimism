package client

import (
	"context"

	grpc "github.com/ethereum-optimism/optimism/shutter-node/grpc/v1"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/shutter-network/shutter/shlib/shcrypto"
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
func (c *Client) GetKey(ctx context.Context, block uint) (*DecryptionKeyResult, error) {
	req := &grpc.GetDecryptionKeyRequest{
		Block: uint64(block),
	}
	opts := []googrpc.CallOption{}

	ok := c.waitState(ctx)
	if !ok {
		// ctx done, or rpc shutting down
		// TODO: error message
		return nil, errors.New("wait state failed")
	}

	resp, err := c.client.GetDecryptionKey(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	decrKey := resp.GetDecryptionKey()
	if resp == nil {
		// TODO:
		return nil, errors.New("no value returned")
	}

	k := &DecryptionKeyResult{
		Block:  uint(decrKey.Block),
		Active: decrKey.Active,
	}

	key := &shcrypto.EpochSecretKey{}
	if err := key.Unmarshal(decrKey.Key); err != nil {
		return nil, errors.Wrap(err, "marshal epoch secret key")
	}
	k.SecretKey = key
	return k, nil
}
