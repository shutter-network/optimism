package client

import (
	"context"

	grpc "github.com/ethereum-optimism/optimism/shutter-node/grpc/v1"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	googrpc "google.golang.org/grpc"
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

// func (c *Client) Start(ctx context.Context, runner service.Runner) error {
// 	err := c.Init(ctx)
// 	if err != nil {
// 		return errors.Wrap(err, "initialize grpc-client")
// 	}
// 	// TODO: spawn anything to keep running?
// 	// even an atomic safeguarding making requests while the connection is open?
// 	runner.Defer(func() {
// 		c.Close()
// 	})
// 	return nil
// }

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) receiveKeys(ctx context.Context, stream chan<- *DecryptionKeyResult, latestBlock uint) {
	defer close(stream)
	err := c.streamDecryptionKeys(ctx, latestBlock, stream)

	// send the error and close
	val := &DecryptionKeyResult{
		Error: err,
	}
	select {
	case <-ctx.Done():
	// don't send, the context has been canceled from
	// the outside
	case stream <- val:
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
		ok := c.waitState(ctx)
		if !ok {
			// ctx done, or rpc shutting down
			return
		}

		defer close(keyChan)
		resp, err := c.client.GetDecryptionKey(ctx, req, opts...)
		// XXX: although we waited for a state, this can still fail again
		// due to the state. We don't currently catch this and try again..
		// --> is this transparently retrying?
		decrKey := ToResult(resp.GetDecryptionKey(), err)
		// promise, so shouldn't be blocking unlesss we got
		// a send somewhere else
		select {
		case <-ctx.Done():
			return
		case keyChan <- &decrKey:
		}
	}(ctx, key)

	return key
}

func (c *Client) StartReceiveKeys(ctx context.Context, latestBlock uint) <-chan *DecryptionKeyResult {
	stream := make(chan *DecryptionKeyResult)
	go func(stream chan *DecryptionKeyResult) {
		defer close(stream)
		err := c.streamDecryptionKeys(ctx, latestBlock, stream)

		// send the error and close
		val := &DecryptionKeyResult{
			Error: err,
		}
		select {
		case <-ctx.Done():
		// don't send, the context has been canceled from
		// the outside
		case stream <- val:
		}
	}(stream)
	return stream
}

func (c *Client) streamDecryptionKeys(
	ctx context.Context,
	startBlock uint,
	stream chan<- *DecryptionKeyResult,
) error {
	req := &grpc.DecryptionKeyRequest{
		StartBlock: uint64(startBlock),
	}

	decrResponseStream, err := c.client.DecryptionKey(ctx, req)
	if err != nil {
		return err
	}
	defer func() {
		// XXX: can this block?
		err := decrResponseStream.CloseSend()
		if err != nil {
			// TODO: log, but we can't do anythin
			_ = err
		}
	}()
	expectedCounter := uint64(1)
	for {
		resp, err := decrResponseStream.Recv()
		if err != nil {
			return err
		}
		if resp.Counter != expectedCounter {
			// TODO: recover internally?
			return errors.New("missed message")
		}
		key := &shcrypto.EpochSecretKey{}
		err = key.Unmarshal(resp.DecryptionKey.Key)
		if err != nil {
			// is this a reason to return and kill the stream?
			return errors.Wrap(err, "unmarshal key")
		}
		val := &DecryptionKeyResult{
			Block:     uint(resp.DecryptionKey.Block),
			SecretKey: key,
			Error:     nil,
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case stream <- val:
		}
		expectedCounter++
	}
}
