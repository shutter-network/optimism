package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/log"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/opio"
	shutternode "github.com/ethereum-optimism/optimism/shutter-node"
	"github.com/ethereum-optimism/optimism/shutter-node/flags"
	"github.com/ethereum-optimism/optimism/shutter-node/node"
	"github.com/ethereum-optimism/optimism/shutter-node/version"
)

var (
	GitCommit = ""
	GitDate   = ""
)

// VersionWithMeta holds the textual version string including the metadata.
var VersionWithMeta = opservice.FormatVersion(version.Version, GitCommit, GitDate, version.Meta)

func main() {
	// Set up logger with a default INFO level in case we fail to parse flags,
	// otherwise the final critical log won't show what the parsing error was.
	oplog.SetupDefaults()

	app := cli.NewApp()
	app.Version = VersionWithMeta
	app.Flags = cliapp.ProtectFlags(flags.Flags)
	app.Name = "shutter-node"
	app.Usage = "Shutter decryption key listener node"
	app.Description = ""
	app.Action = cliapp.LifecycleCmd(ShutterNodeMain)
	// app.Commands = []*cli.Command{}
	ctx := opio.WithInterruptBlocker(context.Background())
	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Crit("Application failed", "message", err)
	}
}

func ShutterNodeMain(ctx *cli.Context, closeApp context.CancelCauseFunc) (cliapp.Lifecycle, error) {
	logCfg := oplog.ReadCLIConfig(ctx)
	log := oplog.NewLogger(oplog.AppOut(ctx), logCfg)
	oplog.SetGlobalLogHandler(log.GetHandler())
	opservice.ValidateEnvVars(flags.EnvVarPrefix, flags.Flags, log)

	cfg, err := shutternode.NewConfig(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("unable to create the rollup node config: %w", err)
	}
	cfg.Cancel = closeApp

	n, err := node.New(ctx.Context, cfg, log, VersionWithMeta)
	if err != nil {
		return nil, fmt.Errorf("unable to create the rollup node: %w", err)
	}

	return n, nil
}
