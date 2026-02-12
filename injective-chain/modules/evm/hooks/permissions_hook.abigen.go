// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package hooks

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
	Bin: "0x6055604b600b8282823980515f1a607314603f577f4e487b71000000000000000000000000000000000000000000000000000000005f525f60045260245ffd5b305f52607381538281f3fe730000000000000000000000000000000000000000301460806040525f5ffdfea2646970667358221220ffccaecd4071c87b214bd01e6e486d71b4310dfe831491c56fc0e6bb48815ae564736f6c634300081e0033",
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
	Bin: "0x6080604052348015600e575f5ffd5b5060e180601a5f395ff3fe6080604052348015600e575f5ffd5b50600436106026575f3560e01c80632ff6e5df14602a575b5f5ffd5b60406004803603810190603c9190606c565b6042565b005b50565b5f5ffd5b5f5ffd5b5f5ffd5b5f604082840312156063576062604d565b5b81905092915050565b5f60208284031215607e57607d6045565b5b5f82013567ffffffffffffffff81111560985760976049565b5b60a2848285016051565b9150509291505056fea2646970667358221220f6e95ff46c5d69a1752bd6d1a29558fa248df799380004bfd849b154589a001e64736f6c634300081e0033",
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

// IPermissionsHookMetaData contains all meta data concerning the IPermissionsHook contract.
var IPermissionsHookMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin\",\"name\":\"amount\",\"type\":\"tuple\"}],\"name\":\"isTransferRestricted\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// IPermissionsHookABI is the input ABI used to generate the binding from.
// Deprecated: Use IPermissionsHookMetaData.ABI instead.
var IPermissionsHookABI = IPermissionsHookMetaData.ABI

// IPermissionsHook is an auto generated Go binding around an Ethereum contract.
type IPermissionsHook struct {
	IPermissionsHookCaller     // Read-only binding to the contract
	IPermissionsHookTransactor // Write-only binding to the contract
	IPermissionsHookFilterer   // Log filterer for contract events
}

// IPermissionsHookCaller is an auto generated read-only Go binding around an Ethereum contract.
type IPermissionsHookCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IPermissionsHookTransactor is an auto generated write-only Go binding around an Ethereum contract.
type IPermissionsHookTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IPermissionsHookFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type IPermissionsHookFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IPermissionsHookSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IPermissionsHookSession struct {
	Contract     *IPermissionsHook // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IPermissionsHookCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IPermissionsHookCallerSession struct {
	Contract *IPermissionsHookCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// IPermissionsHookTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IPermissionsHookTransactorSession struct {
	Contract     *IPermissionsHookTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// IPermissionsHookRaw is an auto generated low-level Go binding around an Ethereum contract.
type IPermissionsHookRaw struct {
	Contract *IPermissionsHook // Generic contract binding to access the raw methods on
}

// IPermissionsHookCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IPermissionsHookCallerRaw struct {
	Contract *IPermissionsHookCaller // Generic read-only contract binding to access the raw methods on
}

// IPermissionsHookTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IPermissionsHookTransactorRaw struct {
	Contract *IPermissionsHookTransactor // Generic write-only contract binding to access the raw methods on
}

// NewIPermissionsHook creates a new instance of IPermissionsHook, bound to a specific deployed contract.
func NewIPermissionsHook(address common.Address, backend bind.ContractBackend) (*IPermissionsHook, error) {
	contract, err := bindIPermissionsHook(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &IPermissionsHook{IPermissionsHookCaller: IPermissionsHookCaller{contract: contract}, IPermissionsHookTransactor: IPermissionsHookTransactor{contract: contract}, IPermissionsHookFilterer: IPermissionsHookFilterer{contract: contract}}, nil
}

// NewIPermissionsHookCaller creates a new read-only instance of IPermissionsHook, bound to a specific deployed contract.
func NewIPermissionsHookCaller(address common.Address, caller bind.ContractCaller) (*IPermissionsHookCaller, error) {
	contract, err := bindIPermissionsHook(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &IPermissionsHookCaller{contract: contract}, nil
}

// NewIPermissionsHookTransactor creates a new write-only instance of IPermissionsHook, bound to a specific deployed contract.
func NewIPermissionsHookTransactor(address common.Address, transactor bind.ContractTransactor) (*IPermissionsHookTransactor, error) {
	contract, err := bindIPermissionsHook(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &IPermissionsHookTransactor{contract: contract}, nil
}

// NewIPermissionsHookFilterer creates a new log filterer instance of IPermissionsHook, bound to a specific deployed contract.
func NewIPermissionsHookFilterer(address common.Address, filterer bind.ContractFilterer) (*IPermissionsHookFilterer, error) {
	contract, err := bindIPermissionsHook(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &IPermissionsHookFilterer{contract: contract}, nil
}

// bindIPermissionsHook binds a generic wrapper to an already deployed contract.
func bindIPermissionsHook(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := IPermissionsHookMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IPermissionsHook *IPermissionsHookRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IPermissionsHook.Contract.IPermissionsHookCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IPermissionsHook *IPermissionsHookRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IPermissionsHook.Contract.IPermissionsHookTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IPermissionsHook *IPermissionsHookRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IPermissionsHook.Contract.IPermissionsHookTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IPermissionsHook *IPermissionsHookCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IPermissionsHook.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IPermissionsHook *IPermissionsHookTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IPermissionsHook.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IPermissionsHook *IPermissionsHookTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IPermissionsHook.Contract.contract.Transact(opts, method, params...)
}

// IsTransferRestricted is a free data retrieval call binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address from, address to, (uint256,string) amount) view returns(bool)
func (_IPermissionsHook *IPermissionsHookCaller) IsTransferRestricted(opts *bind.CallOpts, from common.Address, to common.Address, amount CosmosCoin) (bool, error) {
	var out []interface{}
	err := _IPermissionsHook.contract.Call(opts, &out, "isTransferRestricted", from, to, amount)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsTransferRestricted is a free data retrieval call binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address from, address to, (uint256,string) amount) view returns(bool)
func (_IPermissionsHook *IPermissionsHookSession) IsTransferRestricted(from common.Address, to common.Address, amount CosmosCoin) (bool, error) {
	return _IPermissionsHook.Contract.IsTransferRestricted(&_IPermissionsHook.CallOpts, from, to, amount)
}

// IsTransferRestricted is a free data retrieval call binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address from, address to, (uint256,string) amount) view returns(bool)
func (_IPermissionsHook *IPermissionsHookCallerSession) IsTransferRestricted(from common.Address, to common.Address, amount CosmosCoin) (bool, error) {
	return _IPermissionsHook.Contract.IsTransferRestricted(&_IPermissionsHook.CallOpts, from, to, amount)
}

// PermissionsHookMetaData contains all meta data concerning the PermissionsHook contract.
var PermissionsHookMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"}],\"internalType\":\"structCosmos.Coin\",\"name\":\"amount\",\"type\":\"tuple\"}],\"name\":\"isTransferRestricted\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// PermissionsHookABI is the input ABI used to generate the binding from.
// Deprecated: Use PermissionsHookMetaData.ABI instead.
var PermissionsHookABI = PermissionsHookMetaData.ABI

// PermissionsHook is an auto generated Go binding around an Ethereum contract.
type PermissionsHook struct {
	PermissionsHookCaller     // Read-only binding to the contract
	PermissionsHookTransactor // Write-only binding to the contract
	PermissionsHookFilterer   // Log filterer for contract events
}

// PermissionsHookCaller is an auto generated read-only Go binding around an Ethereum contract.
type PermissionsHookCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PermissionsHookTransactor is an auto generated write-only Go binding around an Ethereum contract.
type PermissionsHookTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PermissionsHookFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type PermissionsHookFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PermissionsHookSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type PermissionsHookSession struct {
	Contract     *PermissionsHook  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PermissionsHookCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type PermissionsHookCallerSession struct {
	Contract *PermissionsHookCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// PermissionsHookTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type PermissionsHookTransactorSession struct {
	Contract     *PermissionsHookTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// PermissionsHookRaw is an auto generated low-level Go binding around an Ethereum contract.
type PermissionsHookRaw struct {
	Contract *PermissionsHook // Generic contract binding to access the raw methods on
}

// PermissionsHookCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type PermissionsHookCallerRaw struct {
	Contract *PermissionsHookCaller // Generic read-only contract binding to access the raw methods on
}

// PermissionsHookTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type PermissionsHookTransactorRaw struct {
	Contract *PermissionsHookTransactor // Generic write-only contract binding to access the raw methods on
}

// NewPermissionsHook creates a new instance of PermissionsHook, bound to a specific deployed contract.
func NewPermissionsHook(address common.Address, backend bind.ContractBackend) (*PermissionsHook, error) {
	contract, err := bindPermissionsHook(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &PermissionsHook{PermissionsHookCaller: PermissionsHookCaller{contract: contract}, PermissionsHookTransactor: PermissionsHookTransactor{contract: contract}, PermissionsHookFilterer: PermissionsHookFilterer{contract: contract}}, nil
}

// NewPermissionsHookCaller creates a new read-only instance of PermissionsHook, bound to a specific deployed contract.
func NewPermissionsHookCaller(address common.Address, caller bind.ContractCaller) (*PermissionsHookCaller, error) {
	contract, err := bindPermissionsHook(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PermissionsHookCaller{contract: contract}, nil
}

// NewPermissionsHookTransactor creates a new write-only instance of PermissionsHook, bound to a specific deployed contract.
func NewPermissionsHookTransactor(address common.Address, transactor bind.ContractTransactor) (*PermissionsHookTransactor, error) {
	contract, err := bindPermissionsHook(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PermissionsHookTransactor{contract: contract}, nil
}

// NewPermissionsHookFilterer creates a new log filterer instance of PermissionsHook, bound to a specific deployed contract.
func NewPermissionsHookFilterer(address common.Address, filterer bind.ContractFilterer) (*PermissionsHookFilterer, error) {
	contract, err := bindPermissionsHook(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PermissionsHookFilterer{contract: contract}, nil
}

// bindPermissionsHook binds a generic wrapper to an already deployed contract.
func bindPermissionsHook(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := PermissionsHookMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_PermissionsHook *PermissionsHookRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _PermissionsHook.Contract.PermissionsHookCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_PermissionsHook *PermissionsHookRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PermissionsHook.Contract.PermissionsHookTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_PermissionsHook *PermissionsHookRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PermissionsHook.Contract.PermissionsHookTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_PermissionsHook *PermissionsHookCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _PermissionsHook.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_PermissionsHook *PermissionsHookTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PermissionsHook.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_PermissionsHook *PermissionsHookTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PermissionsHook.Contract.contract.Transact(opts, method, params...)
}

// IsTransferRestricted is a free data retrieval call binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address from, address to, (uint256,string) amount) view returns(bool)
func (_PermissionsHook *PermissionsHookCaller) IsTransferRestricted(opts *bind.CallOpts, from common.Address, to common.Address, amount CosmosCoin) (bool, error) {
	var out []interface{}
	err := _PermissionsHook.contract.Call(opts, &out, "isTransferRestricted", from, to, amount)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsTransferRestricted is a free data retrieval call binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address from, address to, (uint256,string) amount) view returns(bool)
func (_PermissionsHook *PermissionsHookSession) IsTransferRestricted(from common.Address, to common.Address, amount CosmosCoin) (bool, error) {
	return _PermissionsHook.Contract.IsTransferRestricted(&_PermissionsHook.CallOpts, from, to, amount)
}

// IsTransferRestricted is a free data retrieval call binding the contract method 0xe4e69baf.
//
// Solidity: function isTransferRestricted(address from, address to, (uint256,string) amount) view returns(bool)
func (_PermissionsHook *PermissionsHookCallerSession) IsTransferRestricted(from common.Address, to common.Address, amount CosmosCoin) (bool, error) {
	return _PermissionsHook.Contract.IsTransferRestricted(&_PermissionsHook.CallOpts, from, to, amount)
}
