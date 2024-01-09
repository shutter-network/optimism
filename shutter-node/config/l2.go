package config

import (
	"time"

	"github.com/ethereum-optimism/optimism/shutter-node/flags"
	"github.com/urfave/cli/v2"
)

// NewL2ClientEndpointConfig returns a pointer to a L2SyncEndpointConfig if the
// flag is set, otherwise nil.
func NewL2ClientEndpointConfig(ctx *cli.Context) ShutterL2ClientEndpointConfig {
	return ShutterL2ClientEndpointConfig{
		L2NodeAddr:   ctx.String(flags.L2UnsafeSyncRPC.Name),
		TrustRPC:     ctx.Bool(flags.L2UnsafeSyncRPCTrustRPCFlag.Name),
		PollDuration: ctx.Duration(flags.L2UnsafeSyncRPCPollIntervalFlag.Name),
	}
}

// type ShutterL2ClientEndpointSetup interface {
// 	Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (cl client.RPC, rpcCfg *sources.ShutterL2ClientConfig, err error)
// 	Check() error
// }

type ShutterL2ClientEndpointConfig struct {
	L2NodeAddr   string
	TrustRPC     bool
	PollDuration time.Duration
}

// var _ ShutterL2ClientEndpointSetup = (*ShutterL2ClientEndpointConfig)(nil)

// Setup creates an RPC client to sync from L2 Unsafe head.
// func (cfg *ShutterL2ClientEndpointConfig) Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (client.RPC, *sources.ShutterL2ClientConfig, error) {
// 	if cfg.L2NodeAddr == "" {
// 		return nil, nil, errors.New("no L2 RPC endpoint configured")
// 	}
//
// 	l2Node, err := client.NewRPC(ctx, log, cfg.L2NodeAddr)
// 	if err != nil {
// 		return nil, nil, err
// 	}
//
// 	return l2Node, sources.ShutterL2ClientDefaultConfig(rollupCfg, cfg.TrustRPC), nil
// }

func (cfg *ShutterL2ClientEndpointConfig) Check() error {
	// empty addr is valid, as it is optional.
	return nil
}
