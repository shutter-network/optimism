package rollup

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	ErrBlockTimeZero        = errors.New("block time cannot be 0")
	ErrMissingGenesisL2Hash = errors.New("genesis L2 hash cannot be empty")
	ErrMissingGenesisL2Time = errors.New("missing L2 genesis time")
	ErrMissingL2ChainID     = errors.New("L2 chain ID must not be nil")
	ErrL2ChainIDNotPositive = errors.New("L2 chain ID must be non-zero and positive")
)

type Genesis struct {
	// The L2 block the rollup starts from (no transactions, pre-configured state)
	L2 eth.BlockID `json:"l2"`
	// Timestamp of L2 block
	L2Time uint64 `json:"l2_time"`
}

type Config struct {
	// Genesis anchor point of the rollup
	Genesis Genesis `json:"genesis"`
	// Seconds per L2 block
	BlockTime uint64 `json:"block_time"`

	// Required to identify the L2 network and create p2p signatures unique for this chain.
	L2ChainID *big.Int `json:"l2_chain_id"`
}

// ValidateL2Config checks L2 config variables for errors.
func (cfg *Config) ValidateL2Config(ctx context.Context, client L2Client) error {
	// Validate the L2 Client Chain ID
	if err := cfg.CheckL2ChainID(ctx, client); err != nil {
		return err
	}

	// Validate the Rollup L2 Genesis Blockhash
	if err := cfg.CheckL2GenesisBlockHash(ctx, client); err != nil {
		return err
	}

	return nil
}

// TODO:Needed?
func (cfg *Config) TimestampForBlock(blockNumber uint64) uint64 {
	return cfg.Genesis.L2Time + ((blockNumber - cfg.Genesis.L2.Number) * cfg.BlockTime)
}

// TODO:Needed?
func (cfg *Config) TargetBlockNumber(timestamp uint64) (num uint64, err error) {
	// subtract genesis time from timestamp to get the time elapsed since genesis, and then divide that
	// difference by the block time to get the expected L2 block number at the current time. If the
	// unsafe head does not have this block number, then there is a gap in the queue.
	genesisTimestamp := cfg.Genesis.L2Time
	if timestamp < genesisTimestamp {
		return 0, fmt.Errorf("did not reach genesis time (%d) yet", genesisTimestamp)
	}
	wallClockGenesisDiff := timestamp - genesisTimestamp
	// Note: round down, we should not request blocks into the future.
	blocksSinceGenesis := wallClockGenesisDiff / cfg.BlockTime
	return cfg.Genesis.L2.Number + blocksSinceGenesis, nil
}

type L2Client interface {
	ChainID(context.Context) (*big.Int, error)
	L2BlockRefByNumber(context.Context, uint64) (eth.L2BlockRef, error)
}

// CheckL2ChainID checks that the configured L2 chain ID matches the client's chain ID.
func (cfg *Config) CheckL2ChainID(ctx context.Context, client L2Client) error {
	id, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L2 chain ID: %w", err)
	}
	if cfg.L2ChainID.Cmp(id) != 0 {
		return fmt.Errorf("incorrect L2 RPC chain id %d, expected %d", id, cfg.L2ChainID)
	}
	return nil
}

// CheckL2GenesisBlockHash checks that the configured L2 genesis block hash is valid for the given client.
func (cfg *Config) CheckL2GenesisBlockHash(ctx context.Context, client L2Client) error {
	l2GenesisBlockRef, err := client.L2BlockRefByNumber(ctx, cfg.Genesis.L2.Number)
	if err != nil {
		return fmt.Errorf("failed to get L2 genesis blockhash: %w", err)
	}
	if l2GenesisBlockRef.Hash != cfg.Genesis.L2.Hash {
		return fmt.Errorf("incorrect L2 genesis block hash %s, expected %s", l2GenesisBlockRef.Hash, cfg.Genesis.L2.Hash)
	}
	return nil
}

// Check verifies that the given configuration makes sense
func (cfg *Config) Check() error {
	if cfg.BlockTime == 0 {
		return ErrBlockTimeZero
	}
	if cfg.Genesis.L2.Hash == (common.Hash{}) {
		return ErrMissingGenesisL2Hash
	}
	if cfg.Genesis.L2Time == 0 {
		return ErrMissingGenesisL2Time
	}
	if cfg.L2ChainID == nil {
		return ErrMissingL2ChainID
	}
	if cfg.L2ChainID.Sign() < 1 {
		return ErrL2ChainIDNotPositive
	}
	return nil
}

// Description outputs a banner describing the important parts of rollup configuration in a human-readable form.
// Optionally provide a mapping of L2 chain IDs to network names to label the L2 chain with if not unknown.
// The config should be config.Check()-ed before creating a description.
func (c *Config) Description(l2Chains map[string]string) string {
	// Find and report the network the user is running
	var banner string
	networkL2 := ""
	if l2Chains != nil {
		networkL2 = l2Chains[c.L2ChainID.String()]
	}
	if networkL2 == "" {
		networkL2 = "unknown L2"
	}
	banner += fmt.Sprintf("L2 Chain ID: %v (%s)\n", c.L2ChainID, networkL2)
	// Report the genesis configuration
	banner += "Bedrock starting point:\n"
	banner += fmt.Sprintf("  L2 starting time: %d ~ %s\n", c.Genesis.L2Time, fmtTime(c.Genesis.L2Time))
	banner += fmt.Sprintf("  L2 block: %s %d\n", c.Genesis.L2.Hash, c.Genesis.L2.Number)
	return banner
}

// LogDescription outputs a banner describing the important parts of rollup configuration in a log format.
// Optionally provide a mapping of L2 chain IDs to network names to label the L2 chain with if not unknown.
// The config should be config.Check()-ed before creating a description.
func (c *Config) LogDescription(log log.Logger, l2Chains map[string]string) {
	// Find and report the network the user is running
	networkL2 := ""
	if l2Chains != nil {
		networkL2 = l2Chains[c.L2ChainID.String()]
	}
	if networkL2 == "" {
		networkL2 = "unknown L2"
	}
	log.Info("Rollup Config", "l2_chain_id", c.L2ChainID, "l2_network", networkL2,
		"l2_start_time", c.Genesis.L2Time, "l2_block_hash", c.Genesis.L2.Hash.String(),
		"l2_block_number", c.Genesis.L2.Number,
	)
}

func fmtTime(v uint64) string {
	return time.Unix(int64(v), 0).Format(time.UnixDate)
}

type Epoch uint64
