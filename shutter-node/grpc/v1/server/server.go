package server

import (
	context "context"
	"net"

	grpc "github.com/ethereum-optimism/optimism/shutter-node/grpc/v1"
	"github.com/ethereum-optimism/optimism/shutter-node/grpc/v1/errs"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	googrpc "google.golang.org/grpc"
)

type Server struct {
	grpc.UnimplementedDecryptionKeyServiceServer
	options *options
	log     log.Logger

	dkFn keys.RequestDecryptionKey
	serv *googrpc.Server
}

func NewServer(dkFn keys.RequestDecryptionKey, opts ...Option) (*Server, error) {
	o := &options{}
	o.init()
	err := o.apply(opts)
	if err != nil {
		return nil, errors.Wrap(err, "apply options")
	}
	grpcServer := googrpc.NewServer(o.googopts...)
	s := &Server{
		options: o,
		log:     o.log,
		serv:    grpcServer,
		dkFn:    dkFn,
	}
	grpc.RegisterDecryptionKeyServiceServer(s.serv, s)
	return s, nil
}

func (s *Server) Init(context.Context) {
}

func (s *Server) Start(ctx context.Context, runner service.Runner) error {
	s.Init(ctx)
	// NOTE: if on the same machine/container, this can also be used
	// with unix-socket addresses
	lis, err := net.Listen(s.options.listenNetwork, s.options.listenAddress)
	if err != nil {
		return errors.Wrap(err, "net listen")
	}
	runner.Go(func() error {
		return s.serv.Serve(lis)
	})
	runner.Go(func() error {
		<-ctx.Done()

		// Stops accepting new RPCs, but
		// wait's until currently active calls are served.
		//
		// We are only accepting potentially long-blocking
		// incoming requests for blocks of up to 'latest-block+1',
		// and block progress is dependent on the server returning
		// RPC calls with the next key.
		s.serv.GracefulStop()
		// FIXME: requests for very old blocks should fail immediatly
		// and not block long
		// FIXME: if the shutter-node is shut down,
		// the server has to be cancelled first.
		// It will then serve requests of up to latest-block+1,
		// and then wait for decryption keys of up to latest-block+2.
		// Only if those are persisted, the shutter-node can shut down
		// the P2P service and other services.
		// TODO: in addition to that, an authenticated mechanism for
		// the shutter-node to request resends of decryption keys
		// would be desired
		return ctx.Err()
	})

	// TODO: runner.Defer anything?
	return nil
}

func (s *Server) getDecryptionKey(ctx context.Context, block uint) (
	*grpc.DecryptionKey, error,
) {
	resPromise, cancelRequest := s.dkFn(ctx, block)

	select {
	case <-ctx.Done():
		cancelRequest(ctx.Err())
		return nil, errs.Canceled
	case res := <-resPromise:
		if res.Error != nil {
			return nil, errs.Error(res.Error)
		} else if errors.Is(res.Error, keys.ErrNotActive) {
			return &grpc.DecryptionKey{
				Active: false,
				Block:  uint64(res.Block),
			}, nil
		}
		return &grpc.DecryptionKey{
			Active: true,
			Key:    res.SecretKey.Marshal(),
			Block:  uint64(res.Block),
		}, nil
	}
}

// Unary API
func (s *Server) GetDecryptionKey(
	ctx context.Context,
	req *grpc.GetDecryptionKeyRequest,
) (*grpc.GetDecryptionKeyResponse, error) {
	if req == nil {
		return nil, errors.New("got no request")
	}
	block := uint(req.GetBlock())
	s.log.Info("received gRPC call 'GetDecryptionKey'", "block", block)
	// FIXME:
	// If there is no STATE for a block, then it will return immediately
	// Error: rpc error: code = Unknown desc = no block state found
	// But if there is a state, but no key, then the request will block until
	// timeout / forever
	decrKey, err := s.getDecryptionKey(ctx, block)
	defer func() {
		s.log.Info("served gRPC call 'GetDecryptionKey'", "has-key", decrKey != nil, "error", err)
	}()
	if err != nil {
		return nil, err
	}
	return &grpc.GetDecryptionKeyResponse{
		DecryptionKey: decrKey,
	}, nil
}
