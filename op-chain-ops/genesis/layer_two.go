package genesis

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/immutables"
	"github.com/ethereum-optimism/optimism/op-chain-ops/state"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// BuildL2DeveloperGenesis will build the L2 genesis block.
func BuildL2Genesis(config *DeployConfig, l1StartBlock *types.Block) (*core.Genesis, error) {
	genspec, err := NewL2Genesis(config, l1StartBlock)
	if err != nil {
		return nil, err
	}

	db := state.NewMemoryStateDB(genspec)
	if config.FundDevAccounts {
		log.Info("Funding developer accounts in L2 genesis")
		FundDevAccounts(db)
	}

	SetPrecompileBalances(db)

	storage, err := NewL2StorageConfig(config, l1StartBlock)
	if err != nil {
		return nil, err
	}

	immutable, err := NewL2ImmutableConfig(config, l1StartBlock)
	if err != nil {
		return nil, err
	}

	// Set up the proxies
	err = setProxies(db, predeploys.ProxyAdminAddr, BigL2PredeployNamespace, 2048)
	if err != nil {
		return nil, err
	}

	// Set up the implementations
	deployResults, err := immutables.BuildOptimism(immutable)
	if err != nil {
		return nil, err
	}
	// TODO: shutter predeploy
	for name, predeploy := range predeploys.Predeploys {
		addr := *predeploy
		if addr == predeploys.GovernanceTokenAddr && !config.EnableGovernance {
			// there is no governance token configured, so skip the governance token predeploy
			log.Warn("Governance is not enabled, skipping governance token predeploy.")
			continue
		}
		// TODO: skip predeploys if shutter is not enabled
		codeAddr := addr
		// XXX: shutter is a proxy currently
		// From the bedrock contract
		// ### Proxy by Default
		//
		// All contracts should be assumed to live behind proxies (except in certain special circumstances).
		// This means that new contracts MUST be built under the assumption of upgradeability.
		// We use a minimal [`Proxy`](./contracts/universal/Proxy.sol) contract designed to be owned by a
		// corresponding [`ProxyAdmin`](./contracts/universal/ProxyAdmin.sol) which follow the interfaces
		// of OpenZeppelin's `Proxy` and `ProxyAdmin` contracts, respectively.
		//
		// Unless explicitly discussed otherwise, you MUST include the following basic upgradeability
		// pattern for each new implementation contract:
		//
		// 1. Extend OpenZeppelin's `Initializable` base contract.
		// 2. Include a `uint8 public constant VERSION = X` at the TOP of your contract.
		// 3. Include a function `initialize` with the modifier `reinitializer(VERSION)`.
		// 4. In the `constructor`, set any `immutable` variables and call the `initialize` function for setting mutables.
		if predeploys.IsProxied(addr) {
			codeAddr, err = AddressToCodeNamespace(addr)
			if err != nil {
				return nil, fmt.Errorf("error converting to code namespace: %w", err)
			}
			db.CreateAccount(codeAddr)
			db.SetState(addr, ImplementationSlot, eth.AddressAsLeftPaddedHash(codeAddr))
			log.Info("Set proxy", "name", name, "address", addr, "implementation", codeAddr)
		} else {
			db.DeleteState(addr, AdminSlot)
		}
		if err := setupPredeploy(db, deployResults, storage, name, addr, codeAddr); err != nil {
			return nil, err
		}
		code := db.GetCode(codeAddr)
		if len(code) == 0 {
			return nil, fmt.Errorf("code not set for %s", name)
		}
	}

	return db.Genesis(), nil
}
