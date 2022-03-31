package proxyd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
)

var (
	MainnetChainId = big.NewInt(10)
	KovanChainId   = big.NewInt(69)
)

type DaisyChainServer struct {
	rpcServer          *http.Server
	wsServer           *http.Server
	maxBodySize        int64
	authenticatedPaths map[string]string
	epoch1             *Backend
	epoch2             *Backend
	epoch3             *Backend
	epoch4             *Backend
	epoch5             *Backend
	epoch6             *Backend
	chainId            *big.Int
}

// TODO: support "latest" for epoch
type RequestOptions struct {
	Epoch *uint `json:"epoch,omitempty"`
}

var latestEpoch = uint(6)

// TODO: make this generic
func ptr(n hexutil.Uint64) *hexutil.Uint64 {
	return &n
}

func NewDaisyChainServer(backends map[string]*Backend, maxBodySize int64, authenticatedPaths map[string]string) *DaisyChainServer {
	srv := DaisyChainServer{
		epoch1:             backends["epoch1"],
		epoch2:             backends["epoch2"],
		epoch3:             backends["epoch3"],
		epoch4:             backends["epoch4"],
		epoch5:             backends["epoch5"],
		epoch6:             backends["epoch6"],
		maxBodySize:        maxBodySize,
		authenticatedPaths: authenticatedPaths,
	}
	return &srv
}

func StartDaisyChain(config *Config) (func(), error) {
	if err := config.ValidateDaisyChainBackends(); err != nil {
		return nil, err
	}

	// TODO: figure out how to not need to pass
	// in the rate limiter here by parsing the
	// args in the config and creating it in there
	lim := NewLocalRateLimiter()
	_, backendsByName, err := config.BuildBackends(lim)
	if err != nil {
		return nil, err
	}

	resolvedAuth, err := config.ResolveAuth()
	if err != nil {
		return nil, err
	}

	// parse the config
	srv := NewDaisyChainServer(
		backendsByName,
		config.Server.MaxBodySizeBytes,
		resolvedAuth,
	//authenticatedPaths map[string]string,
	)

	// send a chain id request to each node to ensure they are on the same chain
	req, _ := ParseRPCReq([]byte(`{"id":"1","jsonrpc":"2.0","method":"eth_chainId","params":[]}`))
	chainIds := []*hexutil.Big{}
	for _, backend := range srv.Backends() {
		res, _ := backend.Forward(context.Background(), req)
		str, ok := res.Result.(string)
		if !ok {
			return nil, errors.New("cannot fetch chainid on start")
		}
		chainId := new(hexutil.Big)
		_ = chainId.UnmarshalText([]byte(str))
		chainIds = append(chainIds, chainId)
	}

	if len(chainIds) == 0 {
		panic("cannot fetch remote chain id")
	}
	chainId := chainIds[0].ToInt()
	for _, id := range chainIds {
		if id.ToInt().Cmp(chainId) != 0 {
			log.Crit("mismatched chain ids detected", "chain-id", chainId, "other", id)
		}
	}
	log.Info("detected chain id", "value", chainId)
	srv.chainId = chainId

	if config.Metrics.Enabled {
		addr := fmt.Sprintf("%s:%d", config.Metrics.Host, config.Metrics.Port)
		log.Info("starting metrics server", "addr", addr)
		go func() {
			if err := http.ListenAndServe(addr, promhttp.Handler()); err != nil {
				log.Error("error starting metrics server", "err", err)
			}
		}()
	}

	// To allow integration tests to cleanly come up, wait
	// 10ms to give the below goroutines enough time to
	// encounter an error creating their servers
	errTimer := time.NewTimer(10 * time.Millisecond)

	if config.Server.RPCPort != 0 {
		go func() {
			if err := srv.RPCListenAndServe(config.Server.RPCHost, config.Server.RPCPort); err != nil {
				if errors.Is(err, http.ErrServerClosed) {
					log.Info("RPC server shut down")
					return
				}
				log.Crit("error starting RPC server", "err", err)
			}
		}()
	}

	if config.Server.WSPort != 0 {
		go func() {
			if err := srv.WSListenAndServe(config.Server.WSHost, config.Server.WSPort); err != nil {
				if errors.Is(err, http.ErrServerClosed) {
					log.Info("WS server shut down")
					return
				}
				log.Crit("error starting WS server", "err", err)
			}
		}()
	}

	<-errTimer.C
	log.Info("started daisychain")

	return func() {
		log.Info("shutting down daisychain")
		srv.Shutdown()
		log.Info("goodbye")
	}, nil
}

func (s *DaisyChainServer) HandleRPC(w http.ResponseWriter, r *http.Request) {
	ctx := populateContext(w, r, s.authenticatedPaths)
	if ctx == nil {
		return
	}

	doRequest := func(ctx context.Context, req *RPCReq) (*RPCRes, bool) {
		argType, ok := argTypes[req.Method]
		if !ok {
			return NewRPCErrorRes(req.ID, ErrParseErr), false
		}

		values, err := parsePositionalArguments(req.Params, argType)
		if err != nil {
			return NewRPCErrorRes(req.ID, fmt.Errorf("%s: %w", ErrParseErr, err)), false
		}

		argument, ok := parseRequestOptions(values)
		if !ok {
			return NewRPCErrorRes(req.ID, ErrParseErr), false
		}

		req, err = trimRequestOptions(req, values)
		if err != nil {
			return NewRPCErrorRes(req.ID, ErrParseErr), false
		}

		var res *RPCRes
		if s.isLatestEpochsRPC(argument) {
			// Check to see if the request is meant to go for
			// the latest epochs (5 or 6)
			res = s.handleLatestEpochsRPC(ctx, req)
		} else if s.isHashBasedRPC(values) {
			// Check to see if a hash was passed in the rpc params
			res = s.handleHashTaggedRPC(ctx, req)
		} else {
			res = s.handleEpochRPC(ctx, req, argument)
		}
		return res, false
	}

	handleRPC(ctx, w, r, s.maxBodySize, doRequest)
}

func (s *DaisyChainServer) isLatestEpochsRPC(opts *RequestOptions) bool {
	if opts == nil {
		return true
	}
	if opts.Epoch == nil {
		return true
	}
	if *opts.Epoch == 5 || *opts.Epoch == 6 {
		return true
	}
	return false
}

func (s *DaisyChainServer) Backends() []*Backend {
	backends := []*Backend{}
	if s.epoch1 != nil {
		backends = append(backends, s.epoch1)
	}
	if s.epoch2 != nil {
		backends = append(backends, s.epoch2)
	}
	if s.epoch3 != nil {
		backends = append(backends, s.epoch3)
	}
	if s.epoch4 != nil {
		backends = append(backends, s.epoch4)
	}
	if s.epoch5 != nil {
		backends = append(backends, s.epoch5)
	}
	if s.epoch6 != nil {
		backends = append(backends, s.epoch6)
	}
	return backends
}

// TODO: need the blocknumbers to determine 5 or 6
func (s *DaisyChainServer) handleLatestEpochsRPC(ctx context.Context, req *RPCReq) *RPCRes {
	if s.epoch6 == nil {
		return NewRPCErrorRes(req.ID, ErrInternal)
	}
	res, _ := s.epoch6.Forward(ctx, req)
	return res
}

func (s *DaisyChainServer) handleEpochRPC(ctx context.Context, req *RPCReq, argument *RequestOptions) *RPCRes {
	var backend *Backend
	switch *argument.Epoch {
	case 6:
		backend = s.epoch6
	case 5:
		backend = s.epoch5
	case 4:
		backend = s.epoch4
	case 3:
		backend = s.epoch3
	case 2:
		backend = s.epoch2
	case 1:
		backend = s.epoch1
	default:
		return NewRPCErrorRes(req.ID, ErrInternal)
	}

	// This should never happen
	if backend == nil {
		return NewRPCErrorRes(req.ID, ErrInternal)
	}

	res, err := backend.Forward(ctx, req)
	if err != nil {
		return NewRPCErrorRes(req.ID, err)
	}
	return res
}

func (s *DaisyChainServer) isHashBasedRPC(values []reflect.Value) bool {
	for _, value := range values {
		iface := value.Interface()
		if param, ok := iface.(rpc.BlockNumberOrHash); ok {
			if _, ok := param.Hash(); ok {
				return true
			}
		}
		if _, ok := iface.(common.Hash); ok {
			return true
		}
	}
	return false
}

func (s *DaisyChainServer) RPCListenAndServe(host string, port int) error {
	hdlr := mux.NewRouter()
	hdlr.HandleFunc("/healthz", s.HandleHealthz).Methods("GET")
	hdlr.HandleFunc("/", s.HandleRPC).Methods("POST")
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})
	addr := fmt.Sprintf("%s:%d", host, port)
	s.rpcServer = &http.Server{
		Handler: instrumentedHdlr(c.Handler(hdlr)),
		Addr:    addr,
	}
	log.Info("starting HTTP server", "addr", addr)
	return s.rpcServer.ListenAndServe()
}

func (s *DaisyChainServer) WSListenAndServe(host string, port int) error {
	hdlr := mux.NewRouter()
	hdlr.HandleFunc("/healthz", s.HandleHealthz).Methods("GET")
	// TODO: fix
	//hdlr.HandleFunc("/", s.HandleWS)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})
	addr := fmt.Sprintf("%s:%d", host, port)
	s.wsServer = &http.Server{
		Handler: instrumentedHdlr(c.Handler(hdlr)),
		Addr:    addr,
	}
	log.Info("starting WS server", "addr", addr)
	return s.wsServer.ListenAndServe()
}

func (s *DaisyChainServer) Shutdown() {
	if s.rpcServer != nil {
		_ = s.rpcServer.Shutdown(context.Background())
	}
	if s.wsServer != nil {
		_ = s.wsServer.Shutdown(context.Background())
	}
}

func (s *DaisyChainServer) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

// Tries each rpc url one after another
func (s *DaisyChainServer) handleHashTaggedRPC(ctx context.Context, req *RPCReq) *RPCRes {
	backends := s.Backends()
	var res *RPCRes
	for _, backend := range backends {
		res, _ = backend.Forward(ctx, req)
		if !res.IsError() {
			break
		}
	}
	return res
}

func trimRequestOptions(req *RPCReq, values []reflect.Value) (*RPCReq, error) {
	raw, err := json.Marshal(values[0 : len(values)-1])
	if err != nil {
		return nil, err
	}
	req.Params = raw
	return req, nil
}

func parseRequestOptions(values []reflect.Value) (*RequestOptions, bool) {
	requestOpts := values[len(values)-1]
	argument, ok := requestOpts.Interface().(*RequestOptions)
	if !ok {
		return nil, false
	}
	// If the epoch is not set, default to the latest
	if argument != nil && argument.Epoch == nil {
		argument.Epoch = &latestEpoch
	}
	return argument, true
}
