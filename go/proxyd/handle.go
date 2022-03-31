package proxyd

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/ethereum/go-ethereum/log"
)

func handleRPC(ctx context.Context, w http.ResponseWriter, r *http.Request, maxBodySize int64, doRequest func(context.Context, *RPCReq) (*RPCRes, bool)) {
	log.Info(
		"received RPC request",
		"req_id", GetReqID(ctx),
		"auth", GetAuthCtx(ctx),
		"user_agent", r.Header.Get("user-agent"),
	)

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		log.Error("error reading request body", "err", err)
		writeRPCError(ctx, w, nil, ErrInternal)
		return
	}
	RecordRequestPayloadSize(ctx, len(body))

	if IsBatch(body) {
		reqs, err := ParseBatchRPCReq(body)
		if err != nil {
			log.Error("error parsing batch RPC request", "err", err)
			RecordRPCError(ctx, BackendProxyd, MethodUnknown, err)
			writeRPCError(ctx, w, nil, ErrParseErr)
			return
		}

		if len(reqs) > MaxBatchRPCCalls {
			RecordRPCError(ctx, BackendProxyd, MethodUnknown, ErrTooManyBatchRequests)
			writeRPCError(ctx, w, nil, ErrTooManyBatchRequests)
			return
		}

		if len(reqs) == 0 {
			writeRPCError(ctx, w, nil, ErrInvalidRequest("must specify at least one batch call"))
			return
		}

		batchRes := make([]*RPCRes, len(reqs), len(reqs))
		var batchContainsCached bool
		for i := 0; i < len(reqs); i++ {
			req, err := ParseRPCReq(reqs[i])
			if err != nil {
				log.Info("error parsing RPC call", "source", "rpc", "err", err)
				batchRes[i] = NewRPCErrorRes(nil, err)
				continue
			}

			var cached bool
			batchRes[i], cached = doRequest(ctx, req)
			if cached {
				batchContainsCached = true
			}
		}

		setCacheHeader(w, batchContainsCached)
		writeBatchRPCRes(ctx, w, batchRes)
		return
	}

	req, err := ParseRPCReq(body)
	if err != nil {
		log.Info("error parsing RPC call", "source", "rpc", "err", err)
		writeRPCError(ctx, w, nil, err)
		return
	}

	backendRes, cached := doRequest(ctx, req)
	setCacheHeader(w, cached)
	writeRPCRes(ctx, w, backendRes)
}
