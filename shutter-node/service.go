package shutternode

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/shutter-node/config"
	"github.com/ethereum-optimism/optimism/shutter-node/flags"
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

func NewP2PConfig(ctx *cli.Context, log log.Logger) (*shp2p.Config, error) {
	p2pConfig := shp2p.NewConfig()
	// XXX: for now, the default values are used,
	// and thus listen address and environment is not configurable
	// TODO: expose those to the CLI args
	p2pConfig.SetDefaultValues()

	// TODO: parse privkey and bootnodes and set on the config
	p2pPrivKey := ctx.String(flags.P2PPrivteKey.Name)
	bootNodes := ctx.String(flags.P2PBootNodes.Name)
	_ = p2pPrivKey
	_ = bootNodes
	return p2pConfig, nil
}

// NewConfig creates a Config from the provided flags or environment variables.
func NewConfig(ctx *cli.Context, log log.Logger) (*config.Config, error) {
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, err
	}

	rollupConfig, err := NewRollupConfig(log, ctx)
	if err != nil {
		return nil, err
	}

	l2ClientEndpoint := config.NewL2ClientEndpointConfig(ctx)

	p2pConfig, err := NewP2PConfig(ctx, log)
	if err != nil {
		return nil, err
	}

	// TODO: cfg.Cancel
	cfg := &config.Config{
		Rollup: *rollupConfig,
		// TODO: either Network or Rollup config
		// Network
		P2P:    p2pConfig,
		L2Sync: l2ClientEndpoint,
		RPC: config.RPCConfig{
			ListenAddr: ctx.String(flags.RPCListenAddr.Name),
			ListenPort: ctx.Int(flags.RPCListenPort.Name),
		},
		Metrics: config.MetricsConfig{
			Enabled:    ctx.Bool(flags.MetricsEnabledFlag.Name),
			ListenAddr: ctx.String(flags.MetricsAddrFlag.Name),
			ListenPort: ctx.Int(flags.MetricsPortFlag.Name),
		},
	}

	if err := cfg.Check(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func NewRollupConfig(log log.Logger, ctx *cli.Context) (*rollup.Config, error) {
	network := ctx.String(flags.Network.Name)
	rollupConfigPath := ctx.String(flags.RollupConfig.Name)
	if network != "" {
		if rollupConfigPath != "" {
			log.Error(`Cannot configure network and rollup-config at the same time.
Startup will proceed to use the network-parameter and ignore the rollup config.
Conflicting configuration is deprecated, and will stop the op-node from starting in the future.
`, "network", network, "rollup_config", rollupConfigPath)
		}
		config, err := chaincfg.GetRollupConfig(network)
		if err != nil {
			return nil, err
		}
		if ctx.IsSet(flags.CanyonOverrideFlag.Name) {
			canyon := ctx.Uint64(flags.CanyonOverrideFlag.Name)
			config.CanyonTime = &canyon
		}

		return config, nil
	}

	file, err := os.Open(rollupConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rollup config: %w", err)
	}
	defer file.Close()

	var rollupConfig rollup.Config
	if err := json.NewDecoder(file).Decode(&rollupConfig); err != nil {
		return nil, fmt.Errorf("failed to decode rollup config: %w", err)
	}
	if ctx.IsSet(flags.CanyonOverrideFlag.Name) {
		canyon := ctx.Uint64(flags.CanyonOverrideFlag.Name)
		rollupConfig.CanyonTime = &canyon
	}
	return &rollupConfig, nil
}
