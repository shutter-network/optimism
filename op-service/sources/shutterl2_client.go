package sources

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type ShutterSync interface {
	io.Closer
	// Start starts an additional worker syncing job
	Start() error
}

// TODO:remove this module
type ShutterL2Client struct {
	*L2Client

	requests chan uint64

	resCtx    context.Context
	resCancel context.CancelFunc

	wg sync.WaitGroup
}

type ShutterL2ClientConfig struct {
	L2ClientConfig
}

func ShutterL2ClientDefaultConfig(config *rollup.Config, trustRPC bool) *ShutterL2ClientConfig {
	return &ShutterL2ClientConfig{
		*L2ClientDefaultConfig(config, trustRPC),
	}
}

func NewShutterL2Client(client client.RPC, log log.Logger, metrics caching.Metrics, config *ShutterL2ClientConfig) (*ShutterL2Client, error) {
	l2Client, err := NewL2Client(client, log, metrics, &config.L2ClientConfig)
	if err != nil {
		return nil, err
	}
	// This resource context is shared between all workers that may be started
	resCtx, resCancel := context.WithCancel(context.Background())
	return &ShutterL2Client{
		L2Client:  l2Client,
		resCtx:    resCtx,
		resCancel: resCancel,
		requests:  make(chan uint64, 128),
	}, nil
}

// Start starts the syncing background work. This may not be called after Close().
func (s *ShutterL2Client) Start() error {
	// TODO(CLI-3635): we can start multiple event loop runners as workers, to parallelize the work
	s.wg.Add(1)
	go s.eventLoop()
	return nil
}

func (s *ShutterL2Client) GetKeyperSetFor(ctx context.Context, block eth.L2BlockRef) ([]common.Address, error) {
	// TODO: query the keyper set.. or rather subscribe to the events
	_ = s.rollupCfg.ShutterKeyperSetManagerContractAddress
	return nil, nil
}

// Close sends a signal to close all concurrent syncing work.
func (s *ShutterL2Client) Close() error {
	s.resCancel()
	s.wg.Wait()
	return nil
}

// TODO: remove this, but look at how the request/response is done here
func (s *ShutterL2Client) RequestL2Range(ctx context.Context, start, end eth.L2BlockRef) error {
	// Drain previous requests now that we have new information
	for len(s.requests) > 0 {
		select { // in case requests is being read at the same time, don't block on draining it.
		case <-s.requests:
		default:
			break
		}
	}

	endNum := end.Number
	if end == (eth.L2BlockRef{}) {
		n, err := s.rollupCfg.TargetBlockNumber(uint64(time.Now().Unix()))
		if err != nil {
			return err
		}
		if n <= start.Number {
			return nil
		}
		endNum = n
	}

	// TODO(CLI-3635): optimize the by-range fetching with the Engine API payloads-by-range method.

	s.log.Info("Scheduling to fetch trailing missing payloads from backup RPC", "start", start, "end", endNum, "size", endNum-start.Number-1)

	for i := start.Number + 1; i < endNum; i++ {
		select {
		case s.requests <- i:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// eventLoop is the main event loop for the sync client.
func (s *ShutterL2Client) eventLoop() {
	defer s.wg.Done()
	s.log.Info("Starting sync client event loop")

	backoffStrategy := &retry.ExponentialStrategy{
		Min:       1000 * time.Millisecond,
		Max:       20_000 * time.Millisecond,
		MaxJitter: 250 * time.Millisecond,
	}

	for {
		select {
		case <-s.resCtx.Done():
			s.log.Debug("Shutting down RPC sync worker")
			return
		case reqNum := <-s.requests:
			_, err := retry.Do(s.resCtx, 5, backoffStrategy, func() (interface{}, error) {
				// Limit the maximum time for fetching payloads
				ctx, cancel := context.WithTimeout(s.resCtx, time.Second*10)
				defer cancel()
				_ = ctx
				s.log.Info("queried keyper-set-manager", "val")
				// TODO: fetch new blocks, query the contracts for new events etc.
				// We are only fetching one block at a time here.
				return nil, nil
			})
			if err != nil {
				if err == s.resCtx.Err() {
					return
				}
				s.log.Error("failed syncing L2 block via RPC", "err", err, "num", reqNum)
				// Reschedule at end of queue
				select {
				case s.requests <- reqNum:
				default:
					// drop syncing job if we are too busy with sync jobs already.
				}
			}
		}
	}
}
