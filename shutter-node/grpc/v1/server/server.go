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
		// Don't stop with GracefulStop() because we
		// are expecting hanging long-polling connections.
		s.serv.Stop()
		// TODO: stop with  GracefulStop(), and then
		// signal the event loop to close the long running
		// decryption key streams
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
			// XXX: is this the correct Error to check for?
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
	// FIXME:
	// If there is no STATE for a block, then it will return immediately
	// Error: rpc error: code = Unknown desc = no block state found
	// But if there is a state, but no key, then the request will block until
	// timeout / forever
	decrKey, err := s.getDecryptionKey(ctx, block)
	if err != nil {
		return nil, err
	}
	return &grpc.GetDecryptionKeyResponse{
		DecryptionKey: decrKey,
	}, nil
}

// TODO: do this later
// Streaming API
func (s *Server) DecryptionKey(
	req *grpc.DecryptionKeyRequest,
	stream grpc.DecryptionKeyService_DecryptionKeyServer,
) error {
	// this is the per request / stream context
	ctx := stream.Context()
	if req == nil {
		return errors.New("got no request")
	}
	block := uint(req.GetStartBlock())
	counter := uint64(1)
	for {
		decrKey, err := s.getDecryptionKey(ctx, block)
		if err != nil {
			return err
		}
		resp := &grpc.DecryptionKeyResponse{
			Counter:       counter,
			DecryptionKey: decrKey,
		}

		// NOTE:
		// SendMsg blocks until:
		//   - There is sufficient flow control to schedule m with the transport, or
		//   - The stream is done, or
		//   - The stream breaks.
		// SendMsg does not wait until the message is received by the client. An
		// untimely stream closure may result in lost messages.
		err = stream.Send(resp)
		if err != nil {
			// Sending failed, return err and close connection
			return err
		}
		block++
		counter++
	}
}
