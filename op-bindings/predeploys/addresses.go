package predeploys

import (
	"github.com/ethereum/go-ethereum/common"
	shpredeploys "github.com/shutter-network/shop-contracts/predeploy"
)

// TODO - we should get a single toml yaml or json file source of truth in @eth-optimism/bedrock package
// This needs to be kept in sync with @eth-optimism/contracts-ts/wagmi.config.ts which also specifies this
// To improve robustness and maintainability contracts-bedrock should export all addresses
const (
	L2ToL1MessagePasser           = "0x4200000000000000000000000000000000000016"
	DeployerWhitelist             = "0x4200000000000000000000000000000000000002"
	WETH9                         = "0x4200000000000000000000000000000000000006"
	L2CrossDomainMessenger        = "0x4200000000000000000000000000000000000007"
	L2StandardBridge              = "0x4200000000000000000000000000000000000010"
	SequencerFeeVault             = "0x4200000000000000000000000000000000000011"
	OptimismMintableERC20Factory  = "0x4200000000000000000000000000000000000012"
	L1BlockNumber                 = "0x4200000000000000000000000000000000000013"
	GasPriceOracle                = "0x420000000000000000000000000000000000000F"
	L1Block                       = "0x4200000000000000000000000000000000000015"
	GovernanceToken               = "0x4200000000000000000000000000000000000042"
	LegacyMessagePasser           = "0x4200000000000000000000000000000000000000"
	L2ERC721Bridge                = "0x4200000000000000000000000000000000000014"
	OptimismMintableERC721Factory = "0x4200000000000000000000000000000000000017"
	ProxyAdmin                    = "0x4200000000000000000000000000000000000018"
	BaseFeeVault                  = "0x4200000000000000000000000000000000000019"
	L1FeeVault                    = "0x420000000000000000000000000000000000001a"
	SchemaRegistry                = "0x4200000000000000000000000000000000000020"
	EAS                           = "0x4200000000000000000000000000000000000021"
)

var (
	L2ToL1MessagePasserAddr           = common.HexToAddress(L2ToL1MessagePasser)
	DeployerWhitelistAddr             = common.HexToAddress(DeployerWhitelist)
	WETH9Addr                         = common.HexToAddress(WETH9)
	L2CrossDomainMessengerAddr        = common.HexToAddress(L2CrossDomainMessenger)
	L2StandardBridgeAddr              = common.HexToAddress(L2StandardBridge)
	SequencerFeeVaultAddr             = common.HexToAddress(SequencerFeeVault)
	OptimismMintableERC20FactoryAddr  = common.HexToAddress(OptimismMintableERC20Factory)
	L1BlockNumberAddr                 = common.HexToAddress(L1BlockNumber)
	GasPriceOracleAddr                = common.HexToAddress(GasPriceOracle)
	L1BlockAddr                       = common.HexToAddress(L1Block)
	GovernanceTokenAddr               = common.HexToAddress(GovernanceToken)
	LegacyMessagePasserAddr           = common.HexToAddress(LegacyMessagePasser)
	L2ERC721BridgeAddr                = common.HexToAddress(L2ERC721Bridge)
	OptimismMintableERC721FactoryAddr = common.HexToAddress(OptimismMintableERC721Factory)
	ProxyAdminAddr                    = common.HexToAddress(ProxyAdmin)
	BaseFeeVaultAddr                  = common.HexToAddress(BaseFeeVault)
	L1FeeVaultAddr                    = common.HexToAddress(L1FeeVault)
	SchemaRegistryAddr                = common.HexToAddress(SchemaRegistry)
	EASAddr                           = common.HexToAddress(EAS)

	Predeploys = make(map[string]*common.Address)
)

// IsProxied returns true for predeploys that will sit behind a proxy contract
func IsProxied(predeployAddr common.Address) bool {
	switch predeployAddr {
	case WETH9Addr:
	case GovernanceTokenAddr:
	case shpredeploys.KeyperSetManagerAddr:
	case shpredeploys.InboxAddr:
	case shpredeploys.KeyBroadcastContractAddr:
	default:
		return true
	}
	return false
}

func init() {
	Predeploys["L2ToL1MessagePasser"] = &L2ToL1MessagePasserAddr
	Predeploys["DeployerWhitelist"] = &DeployerWhitelistAddr
	Predeploys["WETH9"] = &WETH9Addr
	Predeploys["L2CrossDomainMessenger"] = &L2CrossDomainMessengerAddr
	Predeploys["L2StandardBridge"] = &L2StandardBridgeAddr
	Predeploys["SequencerFeeVault"] = &SequencerFeeVaultAddr
	Predeploys["OptimismMintableERC20Factory"] = &OptimismMintableERC20FactoryAddr
	Predeploys["L1BlockNumber"] = &L1BlockNumberAddr
	Predeploys["GasPriceOracle"] = &GasPriceOracleAddr
	Predeploys["L1Block"] = &L1BlockAddr
	Predeploys["GovernanceToken"] = &GovernanceTokenAddr
	Predeploys["LegacyMessagePasser"] = &LegacyMessagePasserAddr
	Predeploys["L2ERC721Bridge"] = &L2ERC721BridgeAddr
	Predeploys["OptimismMintableERC721Factory"] = &OptimismMintableERC721FactoryAddr
	Predeploys["ProxyAdmin"] = &ProxyAdminAddr
	Predeploys["BaseFeeVault"] = &BaseFeeVaultAddr
	Predeploys["L1FeeVault"] = &L1FeeVaultAddr
	Predeploys["SchemaRegistry"] = &SchemaRegistryAddr
	Predeploys["EAS"] = &EASAddr

	err := shpredeploys.MergeShutterPredeploys(Predeploys)
	if err != nil {
		panic(err)
	}
}
