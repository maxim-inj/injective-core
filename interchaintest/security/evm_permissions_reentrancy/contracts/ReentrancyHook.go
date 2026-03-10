// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// CosmosCoin is an auto generated low-level Go binding around an user-defined struct.
type CosmosCoin struct {
	Amount *big.Int
	Denom  string
}

// CosmosMetaData contains all meta data concerning the Cosmos contract.
var CosmosMetaData = &bind.MetaData{
	ABI: "[]",
	Bin: "0x6055604b600b8282823980515f1a607314603f577f4e487b71000000000000000000000000000000000000000000000000000000005f525f60045260245ffd5b305f52607381538281f3fe730000000000000000000000000000000000000000301460806040525f5ffdfea26469706673582212207093aa66cb0326ba8cca3e311e53883526791d7a092d96d3291a9e30b69ae2e064736f6c634300081e0033",
}

// CosmosABI is the input ABI used to generate the binding from.
// Deprecated: Use CosmosMetaData.ABI instead.
var CosmosABI = CosmosMetaData.ABI

// CosmosBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use CosmosMetaData.Bin instead.
var CosmosBin = CosmosMetaData.Bin

// DeployCosmos deploys a new Ethereum contract, binding an instance of Cosmos to it.
func DeployCosmos(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Cosmos, error) {
	parsed, err := CosmosMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(CosmosBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Cosmos{CosmosCaller: CosmosCaller{contract: contract}, CosmosTransactor: CosmosTransactor{contract: contract}, CosmosFilterer: CosmosFilterer{contract: contract}}, nil
}

// Cosmos is an auto generated Go binding around an Ethereum contract.
type Cosmos struct {
	CosmosCaller     // Read-only binding to the contract
	CosmosTransactor // Write-only binding to the contract
	CosmosFilterer   // Log filterer for contract events
}

// CosmosCaller is an auto generated read-only Go binding around an Ethereum contract.
type CosmosCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CosmosTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CosmosTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CosmosFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CosmosFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CosmosSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CosmosSession struct {
	Contract     *Cosmos           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CosmosCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CosmosCallerSession struct {
	Contract *CosmosCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// CosmosTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CosmosTransactorSession struct {
	Contract     *CosmosTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CosmosRaw is an auto generated low-level Go binding around an Ethereum contract.
type CosmosRaw struct {
	Contract *Cosmos // Generic contract binding to access the raw methods on
}

// CosmosCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CosmosCallerRaw struct {
	Contract *CosmosCaller // Generic read-only contract binding to access the raw methods on
}

// CosmosTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CosmosTransactorRaw struct {
	Contract *CosmosTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCosmos creates a new instance of Cosmos, bound to a specific deployed contract.
func NewCosmos(address common.Address, backend bind.ContractBackend) (*Cosmos, error) {
	contract, err := bindCosmos(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Cosmos{CosmosCaller: CosmosCaller{contract: contract}, CosmosTransactor: CosmosTransactor{contract: contract}, CosmosFilterer: CosmosFilterer{contract: contract}}, nil
}

// NewCosmosCaller creates a new read-only instance of Cosmos, bound to a specific deployed contract.
func NewCosmosCaller(address common.Address, caller bind.ContractCaller) (*CosmosCaller, error) {
	contract, err := bindCosmos(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CosmosCaller{contract: contract}, nil
}

// NewCosmosTransactor creates a new write-only instance of Cosmos, bound to a specific deployed contract.
func NewCosmosTransactor(address common.Address, transactor bind.ContractTransactor) (*CosmosTransactor, error) {
	contract, err := bindCosmos(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CosmosTransactor{contract: contract}, nil
}

// NewCosmosFilterer creates a new log filterer instance of Cosmos, bound to a specific deployed contract.
func NewCosmosFilterer(address common.Address, filterer bind.ContractFilterer) (*CosmosFilterer, error) {
	contract, err := bindCosmos(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CosmosFilterer{contract: contract}, nil
}

// bindCosmos binds a generic wrapper to an already deployed contract.
func bindCosmos(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := CosmosMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Cosmos *CosmosRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Cosmos.Contract.CosmosCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Cosmos *CosmosRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Cosmos.Contract.CosmosTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Cosmos *CosmosRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Cosmos.Contract.CosmosTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Cosmos *CosmosCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Cosmos.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Cosmos *CosmosTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Cosmos.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Cosmos *CosmosTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Cosmos.Contract.contract.Transact(opts, method, params...)
}

// CosmosTypesMetaData contains all meta data concerning the CosmosTypes contract.
var CosmosTypesMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin\",\"name\":\"\",\"type\":\"tuple\"}],\"name\":\"coin\",\"outputs\":[],\"stateMutability\":\"pure\",\"type\":\"function\"}]",
	Bin: "0x6080604052348015600e575f5ffd5b5060e180601a5f395ff3fe6080604052348015600e575f5ffd5b50600436106026575f3560e01c80632ff6e5df14602a575b5f5ffd5b60406004803603810190603c9190606c565b6042565b005b50565b5f5ffd5b5f5ffd5b5f5ffd5b5f604082840312156063576062604d565b5b81905092915050565b5f60208284031215607e57607d6045565b5b5f82013567ffffffffffffffff81111560985760976049565b5b60a2848285016051565b9150509291505056fea26469706673582212202c87ebe9d25469c0098f6f974c19e5dee6482de0dca43a901b887bf3268285a564736f6c634300081e0033",
}

// CosmosTypesABI is the input ABI used to generate the binding from.
// Deprecated: Use CosmosTypesMetaData.ABI instead.
var CosmosTypesABI = CosmosTypesMetaData.ABI

// CosmosTypesBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use CosmosTypesMetaData.Bin instead.
var CosmosTypesBin = CosmosTypesMetaData.Bin

// DeployCosmosTypes deploys a new Ethereum contract, binding an instance of CosmosTypes to it.
func DeployCosmosTypes(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *CosmosTypes, error) {
	parsed, err := CosmosTypesMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(CosmosTypesBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &CosmosTypes{CosmosTypesCaller: CosmosTypesCaller{contract: contract}, CosmosTypesTransactor: CosmosTypesTransactor{contract: contract}, CosmosTypesFilterer: CosmosTypesFilterer{contract: contract}}, nil
}

// CosmosTypes is an auto generated Go binding around an Ethereum contract.
type CosmosTypes struct {
	CosmosTypesCaller     // Read-only binding to the contract
	CosmosTypesTransactor // Write-only binding to the contract
	CosmosTypesFilterer   // Log filterer for contract events
}

// CosmosTypesCaller is an auto generated read-only Go binding around an Ethereum contract.
type CosmosTypesCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CosmosTypesTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CosmosTypesTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CosmosTypesFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CosmosTypesFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CosmosTypesSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CosmosTypesSession struct {
	Contract     *CosmosTypes      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CosmosTypesCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CosmosTypesCallerSession struct {
	Contract *CosmosTypesCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// CosmosTypesTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CosmosTypesTransactorSession struct {
	Contract     *CosmosTypesTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// CosmosTypesRaw is an auto generated low-level Go binding around an Ethereum contract.
type CosmosTypesRaw struct {
	Contract *CosmosTypes // Generic contract binding to access the raw methods on
}

// CosmosTypesCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CosmosTypesCallerRaw struct {
	Contract *CosmosTypesCaller // Generic read-only contract binding to access the raw methods on
}

// CosmosTypesTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CosmosTypesTransactorRaw struct {
	Contract *CosmosTypesTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCosmosTypes creates a new instance of CosmosTypes, bound to a specific deployed contract.
func NewCosmosTypes(address common.Address, backend bind.ContractBackend) (*CosmosTypes, error) {
	contract, err := bindCosmosTypes(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CosmosTypes{CosmosTypesCaller: CosmosTypesCaller{contract: contract}, CosmosTypesTransactor: CosmosTypesTransactor{contract: contract}, CosmosTypesFilterer: CosmosTypesFilterer{contract: contract}}, nil
}

// NewCosmosTypesCaller creates a new read-only instance of CosmosTypes, bound to a specific deployed contract.
func NewCosmosTypesCaller(address common.Address, caller bind.ContractCaller) (*CosmosTypesCaller, error) {
	contract, err := bindCosmosTypes(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CosmosTypesCaller{contract: contract}, nil
}

// NewCosmosTypesTransactor creates a new write-only instance of CosmosTypes, bound to a specific deployed contract.
func NewCosmosTypesTransactor(address common.Address, transactor bind.ContractTransactor) (*CosmosTypesTransactor, error) {
	contract, err := bindCosmosTypes(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CosmosTypesTransactor{contract: contract}, nil
}

// NewCosmosTypesFilterer creates a new log filterer instance of CosmosTypes, bound to a specific deployed contract.
func NewCosmosTypesFilterer(address common.Address, filterer bind.ContractFilterer) (*CosmosTypesFilterer, error) {
	contract, err := bindCosmosTypes(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CosmosTypesFilterer{contract: contract}, nil
}

// bindCosmosTypes binds a generic wrapper to an already deployed contract.
func bindCosmosTypes(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := CosmosTypesMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CosmosTypes *CosmosTypesRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CosmosTypes.Contract.CosmosTypesCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CosmosTypes *CosmosTypesRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CosmosTypes.Contract.CosmosTypesTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CosmosTypes *CosmosTypesRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CosmosTypes.Contract.CosmosTypesTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CosmosTypes *CosmosTypesCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CosmosTypes.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CosmosTypes *CosmosTypesTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CosmosTypes.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CosmosTypes *CosmosTypesTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CosmosTypes.Contract.contract.Transact(opts, method, params...)
}

// Coin is a free data retrieval call binding the contract method 0x2ff6e5df.
//
// Solidity: function coin((uint256,string) ) pure returns()
func (_CosmosTypes *CosmosTypesCaller) Coin(opts *bind.CallOpts, arg0 CosmosCoin) error {
	var out []interface{}
	err := _CosmosTypes.contract.Call(opts, &out, "coin", arg0)

	if err != nil {
		return err
	}

	return err

}

// Coin is a free data retrieval call binding the contract method 0x2ff6e5df.
//
// Solidity: function coin((uint256,string) ) pure returns()
func (_CosmosTypes *CosmosTypesSession) Coin(arg0 CosmosCoin) error {
	return _CosmosTypes.Contract.Coin(&_CosmosTypes.CallOpts, arg0)
}

// Coin is a free data retrieval call binding the contract method 0x2ff6e5df.
//
// Solidity: function coin((uint256,string) ) pure returns()
func (_CosmosTypes *CosmosTypesCallerSession) Coin(arg0 CosmosCoin) error {
	return _CosmosTypes.Contract.Coin(&_CosmosTypes.CallOpts, arg0)
}

// IBankModuleMetaData contains all meta data concerning the IBankModule contract.
var IBankModuleMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"metadata\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"mint\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"name\":\"setMetadata\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// IBankModuleABI is the input ABI used to generate the binding from.
// Deprecated: Use IBankModuleMetaData.ABI instead.
var IBankModuleABI = IBankModuleMetaData.ABI

// IBankModule is an auto generated Go binding around an Ethereum contract.
type IBankModule struct {
	IBankModuleCaller     // Read-only binding to the contract
	IBankModuleTransactor // Write-only binding to the contract
	IBankModuleFilterer   // Log filterer for contract events
}

// IBankModuleCaller is an auto generated read-only Go binding around an Ethereum contract.
type IBankModuleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IBankModuleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type IBankModuleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IBankModuleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type IBankModuleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IBankModuleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IBankModuleSession struct {
	Contract     *IBankModule      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IBankModuleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IBankModuleCallerSession struct {
	Contract *IBankModuleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// IBankModuleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IBankModuleTransactorSession struct {
	Contract     *IBankModuleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// IBankModuleRaw is an auto generated low-level Go binding around an Ethereum contract.
type IBankModuleRaw struct {
	Contract *IBankModule // Generic contract binding to access the raw methods on
}

// IBankModuleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IBankModuleCallerRaw struct {
	Contract *IBankModuleCaller // Generic read-only contract binding to access the raw methods on
}

// IBankModuleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IBankModuleTransactorRaw struct {
	Contract *IBankModuleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewIBankModule creates a new instance of IBankModule, bound to a specific deployed contract.
func NewIBankModule(address common.Address, backend bind.ContractBackend) (*IBankModule, error) {
	contract, err := bindIBankModule(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &IBankModule{IBankModuleCaller: IBankModuleCaller{contract: contract}, IBankModuleTransactor: IBankModuleTransactor{contract: contract}, IBankModuleFilterer: IBankModuleFilterer{contract: contract}}, nil
}

// NewIBankModuleCaller creates a new read-only instance of IBankModule, bound to a specific deployed contract.
func NewIBankModuleCaller(address common.Address, caller bind.ContractCaller) (*IBankModuleCaller, error) {
	contract, err := bindIBankModule(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &IBankModuleCaller{contract: contract}, nil
}

// NewIBankModuleTransactor creates a new write-only instance of IBankModule, bound to a specific deployed contract.
func NewIBankModuleTransactor(address common.Address, transactor bind.ContractTransactor) (*IBankModuleTransactor, error) {
	contract, err := bindIBankModule(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &IBankModuleTransactor{contract: contract}, nil
}

// NewIBankModuleFilterer creates a new log filterer instance of IBankModule, bound to a specific deployed contract.
func NewIBankModuleFilterer(address common.Address, filterer bind.ContractFilterer) (*IBankModuleFilterer, error) {
	contract, err := bindIBankModule(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &IBankModuleFilterer{contract: contract}, nil
}

// bindIBankModule binds a generic wrapper to an already deployed contract.
func bindIBankModule(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := IBankModuleMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IBankModule *IBankModuleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IBankModule.Contract.IBankModuleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IBankModule *IBankModuleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IBankModule.Contract.IBankModuleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IBankModule *IBankModuleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IBankModule.Contract.IBankModuleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IBankModule *IBankModuleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IBankModule.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IBankModule *IBankModuleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IBankModule.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IBankModule *IBankModuleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IBankModule.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address , address ) view returns(uint256)
func (_IBankModule *IBankModuleCaller) BalanceOf(opts *bind.CallOpts, arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _IBankModule.contract.Call(opts, &out, "balanceOf", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address , address ) view returns(uint256)
func (_IBankModule *IBankModuleSession) BalanceOf(arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	return _IBankModule.Contract.BalanceOf(&_IBankModule.CallOpts, arg0, arg1)
}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address , address ) view returns(uint256)
func (_IBankModule *IBankModuleCallerSession) BalanceOf(arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	return _IBankModule.Contract.BalanceOf(&_IBankModule.CallOpts, arg0, arg1)
}

// Metadata is a free data retrieval call binding the contract method 0x2ba21572.
//
// Solidity: function metadata(address ) view returns(string, string, uint8)
func (_IBankModule *IBankModuleCaller) Metadata(opts *bind.CallOpts, arg0 common.Address) (string, string, uint8, error) {
	var out []interface{}
	err := _IBankModule.contract.Call(opts, &out, "metadata", arg0)

	if err != nil {
		return *new(string), *new(string), *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)
	out1 := *abi.ConvertType(out[1], new(string)).(*string)
	out2 := *abi.ConvertType(out[2], new(uint8)).(*uint8)

	return out0, out1, out2, err

}

// Metadata is a free data retrieval call binding the contract method 0x2ba21572.
//
// Solidity: function metadata(address ) view returns(string, string, uint8)
func (_IBankModule *IBankModuleSession) Metadata(arg0 common.Address) (string, string, uint8, error) {
	return _IBankModule.Contract.Metadata(&_IBankModule.CallOpts, arg0)
}

// Metadata is a free data retrieval call binding the contract method 0x2ba21572.
//
// Solidity: function metadata(address ) view returns(string, string, uint8)
func (_IBankModule *IBankModuleCallerSession) Metadata(arg0 common.Address) (string, string, uint8, error) {
	return _IBankModule.Contract.Metadata(&_IBankModule.CallOpts, arg0)
}

// TotalSupply is a free data retrieval call binding the contract method 0xe4dc2aa4.
//
// Solidity: function totalSupply(address ) view returns(uint256)
func (_IBankModule *IBankModuleCaller) TotalSupply(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _IBankModule.contract.Call(opts, &out, "totalSupply", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0xe4dc2aa4.
//
// Solidity: function totalSupply(address ) view returns(uint256)
func (_IBankModule *IBankModuleSession) TotalSupply(arg0 common.Address) (*big.Int, error) {
	return _IBankModule.Contract.TotalSupply(&_IBankModule.CallOpts, arg0)
}

// TotalSupply is a free data retrieval call binding the contract method 0xe4dc2aa4.
//
// Solidity: function totalSupply(address ) view returns(uint256)
func (_IBankModule *IBankModuleCallerSession) TotalSupply(arg0 common.Address) (*big.Int, error) {
	return _IBankModule.Contract.TotalSupply(&_IBankModule.CallOpts, arg0)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactor) Burn(opts *bind.TransactOpts, arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _IBankModule.contract.Transact(opts, "burn", arg0, arg1)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleSession) Burn(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _IBankModule.Contract.Burn(&_IBankModule.TransactOpts, arg0, arg1)
}

// Burn is a paid mutator transaction binding the contract method 0x9dc29fac.
//
// Solidity: function burn(address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactorSession) Burn(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _IBankModule.Contract.Burn(&_IBankModule.TransactOpts, arg0, arg1)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactor) Mint(opts *bind.TransactOpts, arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _IBankModule.contract.Transact(opts, "mint", arg0, arg1)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleSession) Mint(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _IBankModule.Contract.Mint(&_IBankModule.TransactOpts, arg0, arg1)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactorSession) Mint(arg0 common.Address, arg1 *big.Int) (*types.Transaction, error) {
	return _IBankModule.Contract.Mint(&_IBankModule.TransactOpts, arg0, arg1)
}

// SetMetadata is a paid mutator transaction binding the contract method 0x37d2c2f4.
//
// Solidity: function setMetadata(string , string , uint8 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactor) SetMetadata(opts *bind.TransactOpts, arg0 string, arg1 string, arg2 uint8) (*types.Transaction, error) {
	return _IBankModule.contract.Transact(opts, "setMetadata", arg0, arg1, arg2)
}

// SetMetadata is a paid mutator transaction binding the contract method 0x37d2c2f4.
//
// Solidity: function setMetadata(string , string , uint8 ) payable returns(bool)
func (_IBankModule *IBankModuleSession) SetMetadata(arg0 string, arg1 string, arg2 uint8) (*types.Transaction, error) {
	return _IBankModule.Contract.SetMetadata(&_IBankModule.TransactOpts, arg0, arg1, arg2)
}

// SetMetadata is a paid mutator transaction binding the contract method 0x37d2c2f4.
//
// Solidity: function setMetadata(string , string , uint8 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactorSession) SetMetadata(arg0 string, arg1 string, arg2 uint8) (*types.Transaction, error) {
	return _IBankModule.Contract.SetMetadata(&_IBankModule.TransactOpts, arg0, arg1, arg2)
}

// Transfer is a paid mutator transaction binding the contract method 0xbeabacc8.
//
// Solidity: function transfer(address , address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactor) Transfer(opts *bind.TransactOpts, arg0 common.Address, arg1 common.Address, arg2 *big.Int) (*types.Transaction, error) {
	return _IBankModule.contract.Transact(opts, "transfer", arg0, arg1, arg2)
}

// Transfer is a paid mutator transaction binding the contract method 0xbeabacc8.
//
// Solidity: function transfer(address , address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleSession) Transfer(arg0 common.Address, arg1 common.Address, arg2 *big.Int) (*types.Transaction, error) {
	return _IBankModule.Contract.Transfer(&_IBankModule.TransactOpts, arg0, arg1, arg2)
}

// Transfer is a paid mutator transaction binding the contract method 0xbeabacc8.
//
// Solidity: function transfer(address , address , uint256 ) payable returns(bool)
func (_IBankModule *IBankModuleTransactorSession) Transfer(arg0 common.Address, arg1 common.Address, arg2 *big.Int) (*types.Transaction, error) {
	return _IBankModule.Contract.Transfer(&_IBankModule.TransactOpts, arg0, arg1, arg2)
}

// ReentrancyHookMetaData contains all meta data concerning the ReentrancyHook contract.
var ReentrancyHookMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin\",\"name\":\"\",\"type\":\"tuple\"}],\"name\":\"isTransferRestricted\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"mintTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"triggerRecursion\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
	Bin: "0x6080604052348015600e575f5ffd5b50335f5f6101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506105478061005b5f395ff3fe608060405260043610610042575f3560e01c80638da5cb5b1461004d578063e4e69baf14610077578063eeb9635c146100b3578063efa099a2146100c957610049565b3661004957005b5f5ffd5b348015610058575f5ffd5b506100616100df565b60405161006e91906102d0565b60405180910390f35b348015610082575f5ffd5b5061009d6004803603810190610098919061033d565b610103565b6040516100aa91906103c3565b60405180910390f35b3480156100be575f5ffd5b506100c761018e565b005b3480156100d4575f5ffd5b506100dd61020f565b005b5f5f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b5f606473ffffffffffffffffffffffffffffffffffffffff1663beabacc8303060016040518463ffffffff1660e01b815260040161014393929190610427565b6020604051808303815f875af115801561015f573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906101839190610486565b505f90509392505050565b606473ffffffffffffffffffffffffffffffffffffffff166340c10f19306103e86040518363ffffffff1660e01b81526004016101cc9291906104ea565b6020604051808303815f875af11580156101e8573d5f5f3e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061020c9190610486565b50565b606473ffffffffffffffffffffffffffffffffffffffff1663beabacc8303060016040518463ffffffff1660e01b815260040161024e93929190610427565b6020604051808303815f875af115801561026a573d5f5f3e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061028e9190610486565b50565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6102ba82610291565b9050919050565b6102ca816102b0565b82525050565b5f6020820190506102e35f8301846102c1565b92915050565b5f5ffd5b5f5ffd5b6102fa816102b0565b8114610304575f5ffd5b50565b5f81359050610315816102f1565b92915050565b5f5ffd5b5f604082840312156103345761033361031b565b5b81905092915050565b5f5f5f60608486031215610354576103536102e9565b5b5f61036186828701610307565b935050602061037286828701610307565b925050604084013567ffffffffffffffff811115610393576103926102ed565b5b61039f8682870161031f565b9150509250925092565b5f8115159050919050565b6103bd816103a9565b82525050565b5f6020820190506103d65f8301846103b4565b92915050565b5f819050919050565b5f819050919050565b5f819050919050565b5f61041161040c610407846103dc565b6103ee565b6103e5565b9050919050565b610421816103f7565b82525050565b5f60608201905061043a5f8301866102c1565b61044760208301856102c1565b6104546040830184610418565b949350505050565b610465816103a9565b811461046f575f5ffd5b50565b5f815190506104808161045c565b92915050565b5f6020828403121561049b5761049a6102e9565b5b5f6104a884828501610472565b91505092915050565b5f819050919050565b5f6104d46104cf6104ca846104b1565b6103ee565b6103e5565b9050919050565b6104e4816104ba565b82525050565b5f6040820190506104fd5f8301856102c1565b61050a60208301846104db565b939250505056fea26469706673582212204052fc849eef10e4e97f7dcdd8286a61da65e155f18dcbef830aea5e94122b9164736f6c634300081e0033",
}

// ReentrancyHookABI is the input ABI used to generate the binding from.
// Deprecated: Use ReentrancyHookMetaData.ABI instead.
var ReentrancyHookABI = ReentrancyHookMetaData.ABI

// ReentrancyHookBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use ReentrancyHookMetaData.Bin instead.
var ReentrancyHookBin = ReentrancyHookMetaData.Bin

// DeployReentrancyHook deploys a new Ethereum contract, binding an instance of ReentrancyHook to it.
func DeployReentrancyHook(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *ReentrancyHook, error) {
	parsed, err := ReentrancyHookMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(ReentrancyHookBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &ReentrancyHook{ReentrancyHookCaller: ReentrancyHookCaller{contract: contract}, ReentrancyHookTransactor: ReentrancyHookTransactor{contract: contract}, ReentrancyHookFilterer: ReentrancyHookFilterer{contract: contract}}, nil
}

// ReentrancyHook is an auto generated Go binding around an Ethereum contract.
type ReentrancyHook struct {
	ReentrancyHookCaller     // Read-only binding to the contract
	ReentrancyHookTransactor // Write-only binding to the contract
	ReentrancyHookFilterer   // Log filterer for contract events
}

// ReentrancyHookCaller is an auto generated read-only Go binding around an Ethereum contract.
type ReentrancyHookCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReentrancyHookTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ReentrancyHookTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReentrancyHookFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ReentrancyHookFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReentrancyHookSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ReentrancyHookSession struct {
	Contract     *ReentrancyHook   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ReentrancyHookCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ReentrancyHookCallerSession struct {
	Contract *ReentrancyHookCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// ReentrancyHookTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ReentrancyHookTransactorSession struct {
	Contract     *ReentrancyHookTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// ReentrancyHookRaw is an auto generated low-level Go binding around an Ethereum contract.
type ReentrancyHookRaw struct {
	Contract *ReentrancyHook // Generic contract binding to access the raw methods on
}

// ReentrancyHookCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ReentrancyHookCallerRaw struct {
	Contract *ReentrancyHookCaller // Generic read-only contract binding to access the raw methods on
}

// ReentrancyHookTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ReentrancyHookTransactorRaw struct {
	Contract *ReentrancyHookTransactor // Generic write-only contract binding to access the raw methods on
}

// NewReentrancyHook creates a new instance of ReentrancyHook, bound to a specific deployed contract.
func NewReentrancyHook(address common.Address, backend bind.ContractBackend) (*ReentrancyHook, error) {
	contract, err := bindReentrancyHook(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ReentrancyHook{ReentrancyHookCaller: ReentrancyHookCaller{contract: contract}, ReentrancyHookTransactor: ReentrancyHookTransactor{contract: contract}, ReentrancyHookFilterer: ReentrancyHookFilterer{contract: contract}}, nil
}

// NewReentrancyHookCaller creates a new read-only instance of ReentrancyHook, bound to a specific deployed contract.
func NewReentrancyHookCaller(address common.Address, caller bind.ContractCaller) (*ReentrancyHookCaller, error) {
	contract, err := bindReentrancyHook(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ReentrancyHookCaller{contract: contract}, nil
}

// NewReentrancyHookTransactor creates a new write-only instance of ReentrancyHook, bound to a specific deployed contract.
func NewReentrancyHookTransactor(address common.Address, transactor bind.ContractTransactor) (*ReentrancyHookTransactor, error) {
	contract, err := bindReentrancyHook(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ReentrancyHookTransactor{contract: contract}, nil
}

// NewReentrancyHookFilterer creates a new log filterer instance of ReentrancyHook, bound to a specific deployed contract.
func NewReentrancyHookFilterer(address common.Address, filterer bind.ContractFilterer) (*ReentrancyHookFilterer, error) {
	contract, err := bindReentrancyHook(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ReentrancyHookFilterer{contract: contract}, nil
}

// bindReentrancyHook binds a generic wrapper to an already deployed contract.
func bindReentrancyHook(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ReentrancyHookMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReentrancyHook *ReentrancyHookRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ReentrancyHook.Contract.ReentrancyHookCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReentrancyHook *ReentrancyHookRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReentrancyHook.Contract.ReentrancyHookTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReentrancyHook *ReentrancyHookRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReentrancyHook.Contract.ReentrancyHookTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReentrancyHook *ReentrancyHookCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ReentrancyHook.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReentrancyHook *ReentrancyHookTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReentrancyHook.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReentrancyHook *ReentrancyHookTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReentrancyHook.Contract.contract.Transact(opts, method, params...)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_ReentrancyHook *ReentrancyHookCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ReentrancyHook.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_ReentrancyHook *ReentrancyHookSession) Owner() (common.Address, error) {
	return _ReentrancyHook.Contract.Owner(&_ReentrancyHook.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_ReentrancyHook *ReentrancyHookCallerSession) Owner() (common.Address, error) {
	return _ReentrancyHook.Contract.Owner(&_ReentrancyHook.CallOpts)
}

// IsTransferRestricted is a paid mutator transaction binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address , address , (uint256,string) ) returns(bool)
func (_ReentrancyHook *ReentrancyHookTransactor) IsTransferRestricted(opts *bind.TransactOpts, arg0 common.Address, arg1 common.Address, arg2 CosmosCoin) (*types.Transaction, error) {
	return _ReentrancyHook.contract.Transact(opts, "isTransferRestricted", arg0, arg1, arg2)
}

// IsTransferRestricted is a paid mutator transaction binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address , address , (uint256,string) ) returns(bool)
func (_ReentrancyHook *ReentrancyHookSession) IsTransferRestricted(arg0 common.Address, arg1 common.Address, arg2 CosmosCoin) (*types.Transaction, error) {
	return _ReentrancyHook.Contract.IsTransferRestricted(&_ReentrancyHook.TransactOpts, arg0, arg1, arg2)
}

// IsTransferRestricted is a paid mutator transaction binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address , address , (uint256,string) ) returns(bool)
func (_ReentrancyHook *ReentrancyHookTransactorSession) IsTransferRestricted(arg0 common.Address, arg1 common.Address, arg2 CosmosCoin) (*types.Transaction, error) {
	return _ReentrancyHook.Contract.IsTransferRestricted(&_ReentrancyHook.TransactOpts, arg0, arg1, arg2)
}

// MintTokens is a paid mutator transaction binding the contract method 0xeeb9635c.
//
// Solidity: function mintTokens() returns()
func (_ReentrancyHook *ReentrancyHookTransactor) MintTokens(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReentrancyHook.contract.Transact(opts, "mintTokens")
}

// MintTokens is a paid mutator transaction binding the contract method 0xeeb9635c.
//
// Solidity: function mintTokens() returns()
func (_ReentrancyHook *ReentrancyHookSession) MintTokens() (*types.Transaction, error) {
	return _ReentrancyHook.Contract.MintTokens(&_ReentrancyHook.TransactOpts)
}

// MintTokens is a paid mutator transaction binding the contract method 0xeeb9635c.
//
// Solidity: function mintTokens() returns()
func (_ReentrancyHook *ReentrancyHookTransactorSession) MintTokens() (*types.Transaction, error) {
	return _ReentrancyHook.Contract.MintTokens(&_ReentrancyHook.TransactOpts)
}

// TriggerRecursion is a paid mutator transaction binding the contract method 0xefa099a2.
//
// Solidity: function triggerRecursion() returns()
func (_ReentrancyHook *ReentrancyHookTransactor) TriggerRecursion(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReentrancyHook.contract.Transact(opts, "triggerRecursion")
}

// TriggerRecursion is a paid mutator transaction binding the contract method 0xefa099a2.
//
// Solidity: function triggerRecursion() returns()
func (_ReentrancyHook *ReentrancyHookSession) TriggerRecursion() (*types.Transaction, error) {
	return _ReentrancyHook.Contract.TriggerRecursion(&_ReentrancyHook.TransactOpts)
}

// TriggerRecursion is a paid mutator transaction binding the contract method 0xefa099a2.
//
// Solidity: function triggerRecursion() returns()
func (_ReentrancyHook *ReentrancyHookTransactorSession) TriggerRecursion() (*types.Transaction, error) {
	return _ReentrancyHook.Contract.TriggerRecursion(&_ReentrancyHook.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_ReentrancyHook *ReentrancyHookTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReentrancyHook.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_ReentrancyHook *ReentrancyHookSession) Receive() (*types.Transaction, error) {
	return _ReentrancyHook.Contract.Receive(&_ReentrancyHook.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_ReentrancyHook *ReentrancyHookTransactorSession) Receive() (*types.Transaction, error) {
	return _ReentrancyHook.Contract.Receive(&_ReentrancyHook.TransactOpts)
}
