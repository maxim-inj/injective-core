package keeper

import (
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	erc20types "github.com/InjectiveLabs/injective-core/injective-chain/modules/erc20/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
)

type Keeper struct {
	storeKey       storetypes.StoreKey
	objectStoreKey storetypes.StoreKey

	bankKeeper types.BankKeeper
	tfKeeper   types.TokenFactoryKeeper
	wasmKeeper types.WasmKeeper
	evmKeeper  types.EvmKeeper

	tfModuleAddress string
	moduleAccounts  map[string]bool
	authority       string

	contractPauseListeners       []types.ContractPauseListener
	contractUnpauseListeners     []types.ContractUnpauseListener
	contractBlacklistListeners   []types.ContractBlacklistListener
	contractUnblacklistListeners []types.ContractUnblacklistListener
}

var (
	enforcedContractsKey = []byte("enforcedContracts")
)

// NewKeeper returns a new instance of the x/tokenfactory keeper
func NewKeeper(
	storeKey storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	tfKeeper types.TokenFactoryKeeper,
	wasmKeeper types.WasmKeeper,
	evmKeeper types.EvmKeeper,
	oKey storetypes.StoreKey,
	tfModuleAddress string,
	moduleAccounts map[string]bool,
	authority string,
) Keeper {
	return Keeper{
		storeKey:        storeKey,
		objectStoreKey:  oKey,
		bankKeeper:      bankKeeper,
		tfKeeper:        tfKeeper,
		wasmKeeper:      wasmKeeper,
		evmKeeper:       evmKeeper,
		tfModuleAddress: tfModuleAddress,
		moduleAccounts:  moduleAccounts,
		authority:       authority,
	}
}

// Logger returns a logger for the x/permissions module
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) getEnforcedRestrictionsEvmContracts(ctx sdk.Context) []*types.EnforcedContract {
	// try to get cached value
	store := ctx.ObjectStore(k.objectStoreKey)
	if val := store.Get(enforcedContractsKey); val != nil {
		return val.([]*types.EnforcedContract) //nolint:revive //ok
	}

	// read from storage, cache and return
	contracts := k.GetParams(ctx).EnforcedRestrictionsEvmContracts
	decodedContracts := make([]*types.EnforcedContract, 0, len(contracts))

	for _, contract := range contracts {
		decodedContract := types.EnforcedContract{
			ContractAddress:    common.HexToAddress(contract.ContractAddress),
			PauseEventId:       crypto.Keccak256Hash([]byte(contract.PauseEventSignature)),
			UnpauseEventId:     crypto.Keccak256Hash([]byte(contract.UnpauseEventSignature)),
			BlacklistEventId:   crypto.Keccak256Hash([]byte(contract.BlacklistEventSignature)),
			UnblacklistEventId: crypto.Keccak256Hash([]byte(contract.UnblacklistEventSignature)),
		}

		decodedContracts = append(decodedContracts, &decodedContract)
	}

	store.Set(enforcedContractsKey, decodedContracts)

	return decodedContracts
}

func (k Keeper) clearCachedEnforcedContracts(ctx sdk.Context) {
	ctx.ObjectStore(k.objectStoreKey).Delete(enforcedContractsKey)
}

func (k Keeper) IsEnforcedRestrictionsDenom(ctx sdk.Context, denom string) bool {
	contracts := k.getEnforcedRestrictionsEvmContracts(ctx)
	for i := range contracts {
		if erc20types.DenomPrefix+contracts[i].ContractAddress.Hex() == denom {
			return true
		}
	}
	return false
}

func (k Keeper) PostTxProcessing(ctx sdk.Context, _ *core.Message, receipt *ethtypes.Receipt) error {
	for _, contract := range k.getEnforcedRestrictionsEvmContracts(ctx) {
		for _, logEntry := range receipt.Logs {
			if err := k.processEnforcedRestrictionsLog(ctx, contract, logEntry); err != nil {
				return err
			}
		}
	}
	return nil
}

func (k Keeper) processEnforcedRestrictionsLog(ctx sdk.Context, contract *types.EnforcedContract, logEntry *ethtypes.Log) error {
	if len(logEntry.Topics) == 0 || logEntry.Address.Cmp(contract.ContractAddress) != 0 {
		return nil
	}

	eventID := logEntry.Topics[0]
	contractAddr := contract.ContractAddress.String()

	switch eventID {
	case contract.PauseEventId:
		return k.handlePauseEvent(ctx, contract, contractAddr)
	case contract.UnpauseEventId:
		return k.handleUnpauseEvent(ctx, contract, contractAddr)
	case contract.BlacklistEventId:
		return k.handleBlacklistEvent(ctx, contract, logEntry, contractAddr)
	case contract.UnblacklistEventId:
		return k.handleUnblacklistEvent(ctx, contract, logEntry, contractAddr)
	default:
		return nil
	}
}

func (k Keeper) handlePauseEvent(ctx sdk.Context, contract *types.EnforcedContract, contractAddr string) error {
	k.Logger(ctx).Info("enforced restrictions token pause is detected", "contract_address", contractAddr)

	for _, l := range k.contractPauseListeners {
		if err := l.OnEnforcedRestrictionsEVMContractPause(ctx, contract.ContractAddress); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) handleUnpauseEvent(ctx sdk.Context, contract *types.EnforcedContract, contractAddr string) error {
	k.Logger(ctx).Info("enforced restrictions token unpause is detected", "contract_address", contractAddr)

	for _, l := range k.contractUnpauseListeners {
		if err := l.OnEnforcedRestrictionsEVMContractUnpause(ctx, contract.ContractAddress); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) handleBlacklistEvent(ctx sdk.Context, contract *types.EnforcedContract, logEntry *ethtypes.Log, contractAddr string) error {
	account, ok := k.extractAccountFromLog(ctx, logEntry, "blacklist", contractAddr)
	if !ok {
		return nil
	}

	k.Logger(ctx).Info("enforced restrictions token blacklist is detected", "contract_address", contractAddr, "account", account.String())

	for _, l := range k.contractBlacklistListeners {
		if err := l.OnEnforcedRestrictionsEVMContractBlacklist(ctx, contract.ContractAddress, account); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) handleUnblacklistEvent(ctx sdk.Context, contract *types.EnforcedContract, logEntry *ethtypes.Log, contractAddr string) error {
	account, ok := k.extractAccountFromLog(ctx, logEntry, "un-blacklist", contractAddr)
	if !ok {
		return nil
	}

	k.Logger(ctx).Info("enforced restrictions token un-blacklist is detected", "contract_address", contractAddr, "account", account.String())

	for _, l := range k.contractUnblacklistListeners {
		if err := l.OnEnforcedRestrictionsEVMContractUnblacklist(ctx, contract.ContractAddress, account); err != nil {
			return err
		}
	}
	return nil
}

// extractAccountFromLog reads the account address from the second topic of the log entry.
// Returns false if the topic is missing.
func (k Keeper) extractAccountFromLog(ctx sdk.Context, logEntry *ethtypes.Log, eventName, contractAddr string) (common.Address, bool) {
	if len(logEntry.Topics) < 2 {
		k.Logger(ctx).Warn("enforced restrictions token "+eventName+" is detected but can't derive the account", "contract_address", contractAddr)
		return common.Address{}, false
	}
	return common.BytesToAddress(logEntry.Topics[1].Bytes()), true
}

func (k *Keeper) AddEnforcedRestrictionsEVMContractPauseListener(l types.ContractPauseListener) {
	k.contractPauseListeners = append(k.contractPauseListeners, l)
}
func (k *Keeper) AddEnforcedRestrictionsEVMContractUnpauseListener(l types.ContractUnpauseListener) {
	k.contractUnpauseListeners = append(k.contractUnpauseListeners, l)
}
func (k *Keeper) AddEnforcedRestrictionsEVMContractBlacklistListener(l types.ContractBlacklistListener) {
	k.contractBlacklistListeners = append(k.contractBlacklistListeners, l)
}
func (k *Keeper) AddEnforcedRestrictionsEVMContractUnblacklistListener(l types.ContractUnblacklistListener) {
	k.contractUnblacklistListeners = append(k.contractUnblacklistListeners, l)
}
