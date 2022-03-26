package proxyd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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
	rpcServer   *http.Server
	wsServer    *http.Server
	maxBodySize int64
	epoch1      *Backend
	epoch2      *Backend
	epoch3      *Backend
	epoch4      *Backend
	epoch5      *Backend
	epoch6      *Backend
	chainId     *big.Int
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

// TODO: also add in debug methods
var argTypes = map[string][]reflect.Type{
	// PublicEthereumAPI
	"eth_gasPrice": []reflect.Type{
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_maxPriorityFeePerGas": []reflect.Type{
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_feeHistory": []reflect.Type{
		reflect.TypeOf(rpc.DecimalOrHex(0)),
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf([]float64{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_syncing": []reflect.Type{
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_chainId": []reflect.Type{
		reflect.TypeOf(&RequestOptions{}),
	},

	// PublicBlockChainAPI
	"eth_blockNumber": []reflect.Type{
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getBalance": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getProof": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf([]string{}),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getHeaderByNumber": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getHeaderByHash": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getBlockByNumber": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(true),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getBlockByHash": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(true),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getUncleByBlockNumberAndIndex": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(hexutil.Uint(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getUncleByBlockHashAndIndex": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(hexutil.Uint(0)),
	},
	"eth_getUncleCountByBlockNumber": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getUncleCountByBlockHash": []reflect.Type{},
	"eth_getCode": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getStorageAt": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf(""),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_call": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&StateOverride{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_estimateGas": []reflect.Type{
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_createAccessList": []reflect.Type{
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&RequestOptions{}),
	},

	// PublicTransactionPoolAPI
	"eth_getBlockTransactionCountByNumber": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getBlockTransactionCountByHash": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getTransactionByBlockNumberAndIndex": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(hexutil.Uint(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getTransactionByBlockHashAndIndex": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(hexutil.Uint(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getRawTransactionByBlockNumberAndIndex": []reflect.Type{
		reflect.TypeOf(rpc.BlockNumber(0)),
		reflect.TypeOf(hexutil.Uint(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getRawTransactionByBlockHashAndIndex": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(hexutil.Uint(0)),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getTransactionCount": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf(rpc.BlockNumberOrHash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getTransactionByHash": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getRawTransactionByHash": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_getTransactionReceipt": []reflect.Type{
		reflect.TypeOf(common.Hash{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_sendTransaction": []reflect.Type{
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_fillTransaction": []reflect.Type{
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_sendRawTransaction": []reflect.Type{
		reflect.TypeOf(hexutil.Bytes{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_sign": []reflect.Type{
		reflect.TypeOf(common.Address{}),
		reflect.TypeOf(hexutil.Bytes{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_signTransaction": []reflect.Type{
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_pendingTransactions": []reflect.Type{
		reflect.TypeOf(&RequestOptions{}),
	},
	"eth_resend": []reflect.Type{
		reflect.TypeOf(TransactionArgs{}),
		reflect.TypeOf(&hexutil.Big{}),
		reflect.TypeOf(ptr(hexutil.Uint64(0))),
		reflect.TypeOf(&RequestOptions{}),
	},

	// TODO: fill these out
	// NewPublicTxPoolAPI
	"txpool_content":     []reflect.Type{},
	"txpool_contentFrom": []reflect.Type{},
	"txpool_status":      []reflect.Type{},
	"txpool_inspect":     []reflect.Type{},
}

func NewDaisyChainServer(backends map[string]*Backend, maxBodySize int64) *DaisyChainServer {
	srv := DaisyChainServer{
		epoch1:      backends["epoch1"],
		epoch2:      backends["epoch2"],
		epoch3:      backends["epoch3"],
		epoch4:      backends["epoch4"],
		epoch5:      backends["epoch5"],
		epoch6:      backends["epoch6"],
		maxBodySize: maxBodySize,
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

	// parse the config
	srv := NewDaisyChainServer(
		backendsByName,
		config.Server.MaxBodySizeBytes,
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
		chainId.UnmarshalText([]byte(str))
		chainIds = append(chainIds, chainId)
	}

	if len(chainIds) == 0 {
		panic("cannot fetch remote chain id")
	}
	chainId := chainIds[0].ToInt()
	for _, id := range chainIds {
		if id.ToInt().Cmp(chainId) != 0 {
			panic("mismatched chain ids detected")
		}
	}
	log.Info("detected chain id", "value", chainId)
	srv.chainId = chainId

	if config.Metrics.Enabled {
		addr := fmt.Sprintf("%s:%d", config.Metrics.Host, config.Metrics.Port)
		log.Info("starting metrics server", "addr", addr)
		go http.ListenAndServe(addr, promhttp.Handler())
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

// TODO: batch support
func (s *DaisyChainServer) HandleRPC(w http.ResponseWriter, r *http.Request) {
	ctx := s.populateContext(w, r)
	if ctx == nil {
		return
	}

	log.Info(
		"received RPC request",
		"req_id", GetReqID(ctx),
		"auth", GetAuthCtx(ctx),
		"user_agent", r.Header.Get("user-agent"),
	)

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, s.maxBodySize))
	if err != nil {
		log.Error("error reading request body", "err", err)
		writeRPCError(ctx, w, nil, ErrInternal)
		return
	}
	RecordRequestPayloadSize(ctx, len(body))

	req, err := ParseRPCReq(body)
	if err != nil {
		log.Info("error parsing RPC call", "source", "rpc", "err", err)
		writeRPCError(ctx, w, nil, err)
		return
	}

	argType, ok := argTypes[req.Method]
	if !ok {
		writeRPCError(ctx, w, req.ID, ErrParseErr)
		return
	}

	values, err := parsePositionalArguments(req.Params, argType)
	if err != nil {
		writeRPCError(ctx, w, req.ID, fmt.Errorf("%w: %w", ErrParseErr, err))
		return
	}

	argument, ok := parseRequestOptions(values)
	if !ok {
		writeRPCError(ctx, w, req.ID, ErrParseErr)
		return
	}

	req, err = trimRequestOptions(req, values)
	if err != nil {
		writeRPCError(ctx, w, req.ID, ErrParseErr)
		return
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

	writeRPCRes(ctx, w, res)
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
		s.rpcServer.Shutdown(context.Background())
	}
	if s.wsServer != nil {
		s.wsServer.Shutdown(context.Background())
	}
}

func (s *DaisyChainServer) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func (s *DaisyChainServer) populateContext(w http.ResponseWriter, r *http.Request) context.Context {
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		ipPort := strings.Split(r.RemoteAddr, ":")
		if len(ipPort) == 2 {
			xff = ipPort[0]
		}
	}

	ctx := context.WithValue(r.Context(), ContextKeyXForwardedFor, xff)
	return context.WithValue(
		ctx,
		ContextKeyReqID,
		randStr(10),
	)
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

// TODO: move these helpers to their own file

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	var args []reflect.Value
	tok, err := dec.Token()
	switch {
	case err == io.EOF || tok == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null even though it's
		// not in the spec because our own client used to send it.
	case err != nil:
		return nil, err
	case tok == json.Delim('['):
		// Read argument array.
		if args, err = parseArgumentArray(dec, types); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("non-array args")
	}
	// Set any missing args to nil.
	for i := len(args); i < len(types); i++ {
		if types[i].Kind() != reflect.Ptr {
			return nil, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

func parseArgumentArray(dec *json.Decoder, types []reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return args, fmt.Errorf("too many arguments, want at most %d", len(types))
		}
		argval := reflect.New(types[i])
		if err := dec.Decode(argval.Interface()); err != nil {
			return args, fmt.Errorf("invalid argument %d: %v", i, err)
		}
		if argval.IsNil() && types[i].Kind() != reflect.Ptr {
			return args, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, argval.Elem())
	}
	// Read end of args array.
	_, err := dec.Token()
	return args, err
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

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big      `json:"chainId,omitempty"`
}

// OverrideAccount indicates the overriding fields of account during the execution
// of a message call.
// Note, state and stateDiff can't be specified at the same time. If state is
// set, message execution will only use the data in the given state. Otherwise
// if statDiff is set, all diff will be applied first and then execute the call
// message.
type OverrideAccount struct {
	Nonce     *hexutil.Uint64              `json:"nonce"`
	Code      *hexutil.Bytes               `json:"code"`
	Balance   **hexutil.Big                `json:"balance"`
	State     *map[common.Hash]common.Hash `json:"state"`
	StateDiff *map[common.Hash]common.Hash `json:"stateDiff"`
}

// StateOverride is the collection of overridden accounts.
type StateOverride map[common.Address]OverrideAccount
