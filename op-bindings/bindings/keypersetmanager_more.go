// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const KeyperSetManagerStorageLayoutJSON = "{\"storage\":[{\"astId\":1000,\"contract\":\"src/KeyperSetManager.sol:KeyperSetManager\",\"label\":\"_roles\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_mapping(t_bytes32,t_struct(RoleData)1005_storage)\"},{\"astId\":1001,\"contract\":\"src/KeyperSetManager.sol:KeyperSetManager\",\"label\":\"_paused\",\"offset\":0,\"slot\":\"1\",\"type\":\"t_bool\"},{\"astId\":1002,\"contract\":\"src/KeyperSetManager.sol:KeyperSetManager\",\"label\":\"initializer\",\"offset\":1,\"slot\":\"1\",\"type\":\"t_address\"},{\"astId\":1003,\"contract\":\"src/KeyperSetManager.sol:KeyperSetManager\",\"label\":\"keyperSets\",\"offset\":0,\"slot\":\"2\",\"type\":\"t_array(t_struct(KeyperSetData)1004_storage)dyn_storage\"}],\"types\":{\"t_address\":{\"encoding\":\"inplace\",\"label\":\"address\",\"numberOfBytes\":\"20\"},\"t_array(t_struct(KeyperSetData)1004_storage)dyn_storage\":{\"encoding\":\"dynamic_array\",\"label\":\"struct KeyperSetManager.KeyperSetData[]\",\"numberOfBytes\":\"32\",\"base\":\"t_struct(KeyperSetData)1004_storage\"},\"t_bool\":{\"encoding\":\"inplace\",\"label\":\"bool\",\"numberOfBytes\":\"1\"},\"t_bytes32\":{\"encoding\":\"inplace\",\"label\":\"bytes32\",\"numberOfBytes\":\"32\"},\"t_mapping(t_address,t_bool)\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e bool)\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_bool\"},\"t_mapping(t_bytes32,t_struct(RoleData)1005_storage)\":{\"encoding\":\"mapping\",\"label\":\"mapping(bytes32 =\u003e struct AccessControl.RoleData)\",\"numberOfBytes\":\"32\",\"key\":\"t_bytes32\",\"value\":\"t_struct(RoleData)1005_storage\"},\"t_struct(KeyperSetData)1004_storage\":{\"encoding\":\"inplace\",\"label\":\"struct KeyperSetManager.KeyperSetData\",\"numberOfBytes\":\"32\"},\"t_struct(RoleData)1005_storage\":{\"encoding\":\"inplace\",\"label\":\"struct AccessControl.RoleData\",\"numberOfBytes\":\"64\"},\"t_uint64\":{\"encoding\":\"inplace\",\"label\":\"uint64\",\"numberOfBytes\":\"8\"}}}"

var KeyperSetManagerStorageLayout = new(solc.StorageLayout)

var KeyperSetManagerDeployedBin = "0x608060405234801561001057600080fd5b50600436106101165760003560e01c80638456cb59116100a2578063d3877c4311610071578063d3877c4314610254578063d547741f14610267578063e63ab1e91461027a578063f2e6100a146102a1578063f90f3bed146102a957600080fd5b80638456cb591461020157806391d14854146102095780639ce110d71461021c578063a217fddf1461024c57600080fd5b806336568abe116100e957806336568abe146101b55780633f4ba83a146101c8578063485cc955146101d05780635c975abb146101e3578063636df979146101ee57600080fd5b806301ffc9a71461011b578063035cef1514610143578063248a9ca31461016f5780632f2ff15d146101a0575b600080fd5b61012e610129366004610a7a565b6102bc565b60405190151581526020015b60405180910390f35b610156610151366004610aba565b6102f3565b60405167ffffffffffffffff909116815260200161013a565b61019261017d366004610ad7565b60009081526020819052604090206001015490565b60405190815260200161013a565b6101b36101ae366004610b05565b610381565b005b6101b36101c3366004610b05565b6103ac565b6101b36103e4565b6101b36101de366004610b35565b6103fa565b60015460ff1661012e565b6101566101fc366004610aba565b6104a3565b6101b36104de565b61012e610217366004610b05565b610510565b6001546102349061010090046001600160a01b031681565b6040516001600160a01b03909116815260200161013a565b610192600081565b6101b3610262366004610b63565b610539565b6101b3610275366004610b05565b6107e0565b6101927f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a81565b600254610156565b6102346102b7366004610aba565b610805565b60006001600160e01b03198216637965db0b60e01b14806102ed57506301ffc9a760e01b6001600160e01b03198316145b92915050565b6002546000905b80156103675767ffffffffffffffff83166002610318600184610b97565b8154811061032857610328610baa565b60009182526020909120015467ffffffffffffffff16116103555761034e600182610b97565b9392505050565b8061035f81610bc0565b9150506102fa565b506040516367c9fd1d60e11b815260040160405180910390fd5b60008281526020819052604090206001015461039c81610846565b6103a68383610850565b50505050565b6001600160a01b03811633146103d55760405163334bd91960e11b815260040160405180910390fd5b6103df82826108e2565b505050565b60006103ef81610846565b6103f761094d565b50565b60015461010090046001600160a01b03166104275760405162dc149f60e41b815260040160405180910390fd5b60015461010090046001600160a01b0316331461045757604051630d622feb60e01b815260040160405180910390fd5b610462600083610850565b5061048d7f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a82610850565b505060018054610100600160a81b031916905550565b600060028267ffffffffffffffff16815481106104c2576104c2610baa565b60009182526020909120015467ffffffffffffffff1692915050565b7f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a61050881610846565b6103f761099f565b6000918252602082815260408084206001600160a01b0393909316845291905290205460ff1690565b600061054481610846565b600254158015906105a857506002805461059b919061056590600190610b97565b8154811061057557610575610baa565b60009182526020909120015467ffffffffffffffff16610596436001610bd7565b6109da565b8367ffffffffffffffff16105b156105c6576040516361cb74ab60e11b815260040160405180910390fd5b816001600160a01b0316638d4e40836040518163ffffffff1660e01b8152600401602060405180830381865afa158015610604573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906106289190610bea565b6106455760405163756318d560e11b815260040160405180910390fd5b60408051808201825267ffffffffffffffff80861682526001600160a01b038086166020840181815260028054600181018255600091825295517f405787fa12a823e0f2b7631cc41b3ba8828b3321ca811111fa75cd3aa3bb5ace90960180549251909416600160401b026001600160e01b031990921695909416949094179390931790558251639eab525360e01b8152925185937fa940387dac06ebd336730f1d14b21629a9d137069a9137e871f95313e101016593889386939192639eab525392600480830193928290030181865afa158015610728573d6000803e3d6000fd5b505050506040513d6000823e601f3d908101601f191682016040526107509190810190610c32565b846001600160a01b031663e75235b86040518163ffffffff1660e01b8152600401602060405180830381865afa15801561078e573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906107b29190610cf7565b6002546107c190600190610b97565b6040516107d2959493929190610d14565b60405180910390a150505050565b6000828152602081905260409020600101546107fb81610846565b6103a683836108e2565b600060028267ffffffffffffffff168154811061082457610824610baa565b600091825260209091200154600160401b90046001600160a01b031692915050565b6103f781336109f0565b600061085c8383610510565b6108da576000838152602081815260408083206001600160a01b03861684529091529020805460ff191660011790556108923390565b6001600160a01b0316826001600160a01b0316847f2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d60405160405180910390a45060016102ed565b5060006102ed565b60006108ee8383610510565b156108da576000838152602081815260408083206001600160a01b0386168085529252808320805460ff1916905551339286917ff6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b9190a45060016102ed565b610955610a31565b6001805460ff191690557f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa335b6040516001600160a01b03909116815260200160405180910390a1565b6109a7610a56565b6001805460ff1916811790557f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a25833610982565b60008183116109e9578161034e565b5090919050565b6109fa8282610510565b610a2d5760405163e2517d3f60e01b81526001600160a01b03821660048201526024810183905260440160405180910390fd5b5050565b60015460ff16610a5457604051638dfc202b60e01b815260040160405180910390fd5b565b60015460ff1615610a545760405163d93c066560e01b815260040160405180910390fd5b600060208284031215610a8c57600080fd5b81356001600160e01b03198116811461034e57600080fd5b67ffffffffffffffff811681146103f757600080fd5b600060208284031215610acc57600080fd5b813561034e81610aa4565b600060208284031215610ae957600080fd5b5035919050565b6001600160a01b03811681146103f757600080fd5b60008060408385031215610b1857600080fd5b823591506020830135610b2a81610af0565b809150509250929050565b60008060408385031215610b4857600080fd5b8235610b5381610af0565b91506020830135610b2a81610af0565b60008060408385031215610b7657600080fd5b8235610b5381610aa4565b634e487b7160e01b600052601160045260246000fd5b818103818111156102ed576102ed610b81565b634e487b7160e01b600052603260045260246000fd5b600081610bcf57610bcf610b81565b506000190190565b808201808211156102ed576102ed610b81565b600060208284031215610bfc57600080fd5b8151801515811461034e57600080fd5b634e487b7160e01b600052604160045260246000fd5b8051610c2d81610af0565b919050565b60006020808385031215610c4557600080fd5b825167ffffffffffffffff80821115610c5d57600080fd5b818501915085601f830112610c7157600080fd5b815181811115610c8357610c83610c0c565b8060051b604051601f19603f83011681018181108582111715610ca857610ca8610c0c565b604052918252848201925083810185019188831115610cc657600080fd5b938501935b82851015610ceb57610cdc85610c22565b84529385019392850192610ccb565b98975050505050505050565b600060208284031215610d0957600080fd5b815161034e81610aa4565b600060a0820167ffffffffffffffff8089168452602060018060a01b03808a16602087015260a0604087015283895180865260c08801915060208b01955060005b81811015610d73578651841683529584019591840191600101610d55565b50509783166060870152505093909316608090920191909152509094935050505056fea164736f6c6343000816000a"

func init() {
	if err := json.Unmarshal([]byte(KeyperSetManagerStorageLayoutJSON), KeyperSetManagerStorageLayout); err != nil {
		panic(err)
	}

	layouts["KeyperSetManager"] = KeyperSetManagerStorageLayout
	deployedBytecodes["KeyperSetManager"] = KeyperSetManagerDeployedBin
}
