package keeper

import (
	"fmt"
	"strings"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	erc20types "github.com/InjectiveLabs/injective-core/injective-chain/modules/erc20/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
)

type Keeper struct {
	storeKey storetypes.StoreKey

	bankKeeper types.BankKeeper
	tfKeeper   types.TokenFactoryKeeper
	wasmKeeper types.WasmKeeper
	evmKeeper  types.EvmKeeper

	tfModuleAddress string
	moduleAccounts  map[string]bool
	authority       string
}

// NewKeeper returns a new instance of the x/tokenfactory keeper
func NewKeeper(
	storeKey storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	tfKeeper types.TokenFactoryKeeper,
	wasmKeeper types.WasmKeeper,
	evmKeeper types.EvmKeeper,
	tfModuleAddress string,
	moduleAccounts map[string]bool,
	authority string,
) Keeper {
	return Keeper{
		storeKey:        storeKey,
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

func (k Keeper) IsEnforcedRestrictionsDenom(ctx sdk.Context, denom string) bool {
	params := k.GetParams(ctx)

	if strings.HasPrefix(denom, erc20types.DenomPrefix) && len(params.EnforcedRestrictionsContracts) > 0 {
		targetAddr := ethcommon.HexToAddress(denom[len(erc20types.DenomPrefix):])
		for _, restrictedContract := range params.EnforcedRestrictionsContracts {
			if targetAddr == ethcommon.HexToAddress(restrictedContract) {
				return true
			}
		}
	}
	return false
}
