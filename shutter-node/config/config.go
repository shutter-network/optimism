package config

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type Config struct {
	InstanceID uint64
	L2Sync     ShutterL2ClientEndpointConfig
	Rollup     rollup.Config
	P2P        *shp2p.Config

	// Server config
	RPC     RPCConfig
	Metrics MetricsConfig

	// Cancel to request a premature shutdown of the node itself, e.g. when halting. This may be nil.
	Cancel context.CancelCauseFunc
}

type RPCConfig struct {
	ListenAddr  string
	ListenPort  int
	EnableAdmin bool
}

func (cfg *RPCConfig) HttpEndpoint() string {
	return fmt.Sprintf("http://%s:%d", cfg.ListenAddr, cfg.ListenPort)
}

type MetricsConfig struct {
	Enabled    bool
	ListenAddr string
	ListenPort int
}

func (m MetricsConfig) Check() error {
	if !m.Enabled {
		return nil
	}

	if m.ListenPort < 0 || m.ListenPort > math.MaxUint16 {
		return errors.New("invalid metrics port")
	}

	return nil
}

// Check verifies that the given configuration makes sense
func (cfg *Config) Check() error {
	if err := cfg.L2Sync.Check(); err != nil {
		return fmt.Errorf("sync config error: %w", err)
	}
	// TODO: check p2p config
	if cfg.P2P == nil {
		return fmt.Errorf("no p2p config")
	}
	if err := cfg.Rollup.Check(); err != nil {
		return fmt.Errorf("rollup config error: %w", err)
	}
	if err := cfg.Metrics.Check(); err != nil {
		return fmt.Errorf("metrics config error: %w", err)
	}
	return nil
}
