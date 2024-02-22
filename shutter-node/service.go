package shutternode

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/mitchellh/mapstructure"
	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/shutter-node/config"
	"github.com/ethereum-optimism/optimism/shutter-node/flags"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/address"
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

func mapstructureDecode(input, result any, hookFunc mapstructure.DecodeHookFunc) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			Result:     result,
			DecodeHook: hookFunc,
		})
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}

func MapstructureUnmarshal(input, result any) error {
	return mapstructureDecode(
		input,
		result,
		mapstructure.ComposeDecodeHookFunc(
			medley.TextUnmarshalerHook,
			mapstructure.StringToSliceHookFunc(","),
		),
	)
}

func NewP2PConfig(ctx *cli.Context, log log.Logger) (*shp2p.Config, error) {
	p2pConfig := shp2p.NewConfig()
	p2pConfig.SetDefaultValues()
	// HACK: the mapstructure seems to write to the existing array,
	// and thus if the default array is longer, values at the tail are not
	// removed. We have to remove this manually
	p2pConfig.ListenAddresses = []*address.P2PAddress{}

	p2pPrivKey := ctx.String(flags.P2PPrivateKey.Name)
	bootNodes := ctx.String(flags.P2PBootNodes.Name)
	listenAddrs := ctx.String(flags.P2PListenAddresses.Name)
	input := map[string]string{
		"P2PKey":                   p2pPrivKey,
		"CustomBootstrapAddresses": bootNodes,
		"ListenAddresses":          listenAddrs,
	}
	err := MapstructureUnmarshal(input, p2pConfig)
	if err != nil {
		return nil, err
	}
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

	cfg := &config.Config{
		InstanceID: ctx.Uint64(flags.InstanceID.Name),
		Rollup:     *rollupConfig,
		P2P:        p2pConfig,
		L2Sync:     l2ClientEndpoint,
		RPC: config.RPCConfig{
			ListenAddr: ctx.String(flags.RPCListenAddr.Name),
			ListenPort: ctx.Int(flags.RPCListenPort.Name),
		},
		Metrics: config.MetricsConfig{
			Enabled:    ctx.Bool(flags.MetricsEnabledFlag.Name),
			ListenAddr: ctx.String(flags.MetricsAddrFlag.Name),
			ListenPort: ctx.Int(flags.MetricsPortFlag.Name),
		},
		GRPC: config.GRPCConfig{
			ListenAddress: ctx.String(flags.GRPCListenAddressFlag.Name),
			ListenNetwork: ctx.String(flags.GRPCListenNetworkFlag.Name),
		},
		Database: config.DatabaseConfig{
			FilePath: ctx.String(flags.DatabasePathFlag.Name),
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
