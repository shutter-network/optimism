package client

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/grpc/connectivity"
)

func (c *Client) waitState(ctx context.Context) bool {
	for {
		retry := true
		state := c.conn.GetState()
		switch state {
		case connectivity.Ready:
			retry = false
		case connectivity.Connecting:
		case connectivity.TransientFailure:
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

func (c *Client) outer(ctx context.Context) error {
	startBlock := uint(0)
	for {
		c.waitState(ctx)
		lastBlock, err := c.exhaustStream(ctx, startBlock)
		// TODO: react to the different errors
		if errors.Is(err, ErrFatal) {
			return err
		} else {
			startBlock = lastBlock + 1
			continue
		}
	}
}

var (
	ErrDone   = errors.New("done")
	ErrFatal  = errors.New("fatal")
	ErrClosed = errors.New("closed")
)

func (c *Client) exhaustStream(ctx context.Context, startBlock uint) (uint, error) {
	start := startBlock

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chn := make(chan *DecryptionKeyResult)

	// TODO: go?
	// TODO: close channel
	err := c.streamDecryptionKeys(streamCtx, start, chn)
	if err != nil {
	}

	// receive only cast
	var keyChan <-chan *DecryptionKeyResult = chn
	for {
		select {
		// resources ctx (op-node)
		case <-ctx.Done():
			return start, ErrDone
		case key, ok := <-keyChan:
			if !ok {
				c.log.Error("receive key channel closed")
				return start, ErrClosed
			}
			if key.Error != nil {
				// TODO: if unrecoverable / requires reconnect,
				// return a specific error
				c.log.Error("shutter: grpc decr-key errror", "error", key.Error)
				start++
				cancel()
				continue
			}

			c.log.Info("shutter: got decr-key",
				"key", key.SecretKey.Marshal(),
				"block", key.Block,
			)
			start = key.Block + 1
		}
	}
}

// TODO: split in functions
func (c *Client) StreamKeys(resourcesCtx context.Context, startBlock uint) {
	var start uint = startBlock
outerLoop:
	for {
		retry := true
		state := c.conn.GetState()
		switch state {
		case connectivity.Ready:
			retry = false
		case connectivity.Connecting:
		case connectivity.TransientFailure:
		case connectivity.Idle:
			c.conn.Connect()
		case connectivity.Shutdown:
			c.log.Info("gRPC connection shutting down")
			return
		default:
		}
		if retry {
			c.log.Info("gRPC connection not ready, waiting", "state", state.String())
			c.conn.WaitForStateChange(resourcesCtx, state)
			continue outerLoop
		}
		c.log.Info("gRPC connection READY, requesting key-stream")

		// FIXME:
		otherCtx, cancel := context.WithCancel(context.Background())
		keyChan := c.StartReceiveKeys(otherCtx, start)

	innerLoop:
		for {
			select {
			case <-resourcesCtx.Done():
				cancel()
				return
			case key, ok := <-keyChan:
				if !ok {
					c.log.Error("receive key channel closed")
					cancel()
					continue outerLoop
				}
				if key.Error != nil {
					c.log.Error("shutter: grpc decr-key errror", "error", key.Error)
					start++
					cancel()
					continue innerLoop
				}

				c.log.Info("shutter: got decr-key",
					"key", key.SecretKey.Marshal(),
					"block", key.Block,
				)
				start = key.Block + 1
			}
		}
	}
}
