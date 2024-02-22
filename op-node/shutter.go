package opnode

import (
	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum-optimism/optimism/op-node/shutter"
	"github.com/urfave/cli/v2"
)

func NewShutterConfig(ctx *cli.Context) shutter.Config {
	return shutter.Config{
		ServerAddress: ctx.String(flags.ShutterGRPCAddress.Name),
	}
}
