package flags

import (
	"fmt"
	"strings"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"

	"github.com/urfave/cli/v2"
)

// Flags

const EnvVarPrefix = "OP_SHUTTER"

func prefixEnvVars(name string) []string {
	return []string{EnvVarPrefix + "_" + name}
}

var (
	/* Required Flags */
	L2UnsafeSyncRPC = &cli.StringFlag{
		Name:    "l2.unsafe-sync-rpc",
		Usage:   "Set the L2 unsafe sync RPC endpoint.",
		EnvVars: prefixEnvVars("L2_UNSAFE_SYNC_RPC"),
	}
	RollupConfig = &cli.StringFlag{
		Name:    "rollup.config",
		Usage:   "Rollup chain parameters",
		EnvVars: prefixEnvVars("ROLLUP_CONFIG"),
	}
	Network = &cli.StringFlag{
		Name: "network",
		// TODO: only expose shutter enabled networks
		Usage:   fmt.Sprintf("Predefined network selection. Available networks: %s", strings.Join(chaincfg.AvailableNetworks(), ", ")),
		EnvVars: prefixEnvVars("NETWORK"),
	}
	InstanceID = &cli.Uint64Flag{
		Name:    "instance-id",
		Usage:   "application specific instance-id. Has to match the network the node operates in.",
		EnvVars: prefixEnvVars("INSTANCE_ID"),
	}
	GRPCListenAddressFlag = &cli.StringFlag{
		Name:    "grpc.listen-address",
		Usage:   "gRPC listen address for the decryption key API",
		Value:   ":8282",
		EnvVars: prefixEnvVars("GRPC_LISTEN_ADDRESS"),
	}
	GRPCListenNetworkFlag = &cli.StringFlag{
		Name:    "grpc.listen-network",
		Usage:   "gRPC protocol for the decryption key API",
		Value:   "tcp",
		EnvVars: prefixEnvVars("GRPC_LISTEN_NETWORK"),
	}
	DatabasePathFlag = &cli.PathFlag{
		Name:    "database.path",
		Usage:   "path to the SQLite file",
		Value:   "db.sqlite",
		EnvVars: prefixEnvVars("DATABASE_PATH"),
	}
	P2PBootNodes = &cli.StringFlag{
		Name: "p2p.bootnodes",
		Usage: "Comma-separated multiaddr-format peer list. Connection to trusted PeerEXchange (PX) bootnodes, these peers will be regarded as trusted. " +
			"Addresses of the local peer are ignored. Duplicate/Alternative addresses for the same peer all apply, but only a single connection per peer is established.",
		EnvVars: prefixEnvVars("P2P_BOOTNODES"),
	}
	P2PListenAddresses = &cli.StringFlag{
		Name:    "p2p.listen-addresses",
		Value:   "/ip4/0.0.0.0/tcp/23000",
		Usage:   "Comma-separated multiaddr-format list. Determines on what ports / protocols to listen for.",
		EnvVars: prefixEnvVars("P2P_LISTEN_ADDRS"),
	}
	P2PPrivateKey = &cli.StringFlag{
		Name: "p2p.private-key",
		// TODO: description
		Usage:   "p2p private key",
		EnvVars: prefixEnvVars("P2P_PRIVATE_KEY"),
	}
	/* Optional Flags */
	RPCListenAddr = &cli.StringFlag{
		Name:    "rpc.addr",
		Usage:   "RPC listening address",
		EnvVars: prefixEnvVars("RPC_ADDR"),
		Value:   "127.0.0.1",
	}
	RPCListenPort = &cli.IntFlag{
		Name:    "rpc.port",
		Usage:   "RPC listening port",
		EnvVars: prefixEnvVars("RPC_PORT"),
		Value:   9555,
	}
	MetricsEnabledFlag = &cli.BoolFlag{
		Name:    "metrics.enabled",
		Usage:   "Enable the metrics server",
		EnvVars: prefixEnvVars("METRICS_ENABLED"),
	}
	MetricsAddrFlag = &cli.StringFlag{
		Name:    "metrics.addr",
		Usage:   "Metrics listening address",
		Value:   "0.0.0.0", // TODO(CLI-4159): Switch to 127.0.0.1
		EnvVars: prefixEnvVars("METRICS_ADDR"),
	}
	MetricsPortFlag = &cli.IntFlag{
		Name:    "metrics.port",
		Usage:   "Metrics listening port",
		Value:   7300,
		EnvVars: prefixEnvVars("METRICS_PORT"),
	}
	CanyonOverrideFlag = &cli.Uint64Flag{
		Name:    "override.canyon",
		Usage:   "Manually specify the Canyon fork timestamp, overriding the bundled setting",
		EnvVars: prefixEnvVars("OVERRIDE_CANYON"),
		Hidden:  false,
	}
	L2UnsafeSyncRPCTrustRPCFlag = &cli.BoolFlag{
		Name: "l2.unsafe-sync-rpc.trustrpc",
		Usage: "Configure if response data from the RPC needs to be verified, e.g. blockhash computation." +
			"This does not include checks if the blockhash is part of the canonical chain.",
		EnvVars:  prefixEnvVars("L2_UNSAFE_SYNC_RPC_TRUST_RPC"),
		Required: false,
		Value:    false,
	}
	L2UnsafeSyncRPCPollIntervalFlag = &cli.DurationFlag{
		Name:     "l2.unsafe-sync-rpc.poll-interval",
		Usage:    "Poll frequency (number of polls per block-time) for retrieving new L2 unsafe block updates. Disabled if 0 or negative.",
		EnvVars:  prefixEnvVars("L2_UNSAFE_SYNC_RPC_POLL_INTERVAL"),
		Required: false,
		Value:    500 * time.Millisecond,
	}
	RollupLoadProtocolVersions = &cli.BoolFlag{
		Name:    "rollup.load-protocol-versions",
		Usage:   "Load protocol versions from the superchain L1 ProtocolVersions contract (if available), and report in logs and metrics",
		EnvVars: prefixEnvVars("ROLLUP_LOAD_PROTOCOL_VERSIONS"),
	}
)

var requiredFlags = []cli.Flag{
	RollupConfig,
	L2UnsafeSyncRPC,
	InstanceID,
	P2PBootNodes,
	P2PPrivateKey,
}

var optionalFlags = []cli.Flag{
	Network,
	RPCListenAddr,
	RPCListenPort,
	MetricsEnabledFlag,
	MetricsAddrFlag,
	MetricsPortFlag,
	CanyonOverrideFlag,
	L2UnsafeSyncRPCTrustRPCFlag,
	L2UnsafeSyncRPCPollIntervalFlag,
	RollupLoadProtocolVersions,
	P2PListenAddresses,
	GRPCListenAddressFlag,
	GRPCListenNetworkFlag,
	DatabasePathFlag,
}

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

func init() {
	optionalFlags = append(optionalFlags, oplog.CLIFlags(EnvVarPrefix)...)
	Flags = append(requiredFlags, optionalFlags...)
}

func CheckRequired(ctx *cli.Context) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	return nil
}
