package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetDeposit gets a subaccount's deposit for a given denom.
func (k *BaseKeeper) GetDeposit(
	ctx sdk.Context,
	subaccountID common.Hash,
	denom string,
) *v2.Deposit {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetDepositKey(subaccountID, denom)

	bz := store.Get(key)
	if bz == nil {
		return v2.NewDeposit()
	}

	var deposit v2.Deposit
	k.cdc.MustUnmarshal(bz, &deposit)

	if deposit.TotalBalance.IsNil() {
		deposit.TotalBalance = math.LegacyZeroDec()
	}

	if deposit.AvailableBalance.IsNil() {
		deposit.AvailableBalance = math.LegacyZeroDec()
	}

	return &deposit
}

// SetDeposit sets a subaccount's deposit for a given denom.
func (k *BaseKeeper) SetDeposit(
	ctx sdk.Context,
	subaccountID common.Hash,
	denom string,
	deposit *v2.Deposit,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.SetTransientDeposit(ctx, subaccountID, denom, deposit)

	store := k.getStore(ctx)
	key := types.GetDepositKey(subaccountID, denom)

	// prune from store if deposit is empty
	if deposit == nil || deposit.IsEmpty() {
		store.Delete(key)
		return
	}

	bz := k.cdc.MustMarshal(deposit)
	store.Set(key, bz)
}

// GetDeposits gets all the deposits for all of the subaccount's denoms.
func (k *BaseKeeper) GetDeposits(
	ctx sdk.Context,
	subaccountID common.Hash,
) map[string]*v2.Deposit {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	depositStore := prefix.NewStore(k.getStore(ctx), types.GetDepositKeyPrefixBySubaccountID(subaccountID))
	deposits := make(map[string]*v2.Deposit)

	iterateSafe(depositStore.Iterator(nil, nil), func(key, value []byte) bool {
		denom := string(key)
		var deposit v2.Deposit
		k.cdc.MustUnmarshal(value, &deposit)
		deposits[denom] = &deposit
		return false
	})

	return deposits
}

// GetAllExchangeBalances returns the exchange balances.
func (k *BaseKeeper) GetAllExchangeBalances(
	ctx sdk.Context,
) []v2.Balance {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	depositStore := prefix.NewStore(k.getStore(ctx), types.DepositsPrefix)
	balances := make([]v2.Balance, 0)

	iterateSafe(depositStore.Iterator(nil, nil), func(key, value []byte) bool {
		var deposit v2.Deposit
		k.cdc.MustUnmarshal(value, &deposit)
		subaccountID, denom := types.ParseDepositStoreKey(key)
		balances = append(balances, v2.Balance{
			SubaccountId: subaccountID.Hex(),
			Denom:        denom,
			Deposits:     &deposit,
		})
		return false
	})

	return balances
}

// SetTransientDeposit sets a subaccount's deposit in the transient store for a given denom.
func (k *BaseKeeper) SetTransientDeposit(
	ctx sdk.Context,
	subaccountID common.Hash,
	denom string,
	deposit *v2.Deposit,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	key := types.GetDepositKey(subaccountID, denom)
	bz := k.cdc.MustMarshal(deposit)
	store.Set(key, bz)
}

func (k *BaseKeeper) IterateTransientDeposits(
	ctx sdk.Context,
	process func(subaccountID common.Hash, denom string, deposit *v2.Deposit) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	depositStore := prefix.NewStore(store, types.DepositsPrefix)

	iterateSafe(depositStore.Iterator(nil, nil), func(key, value []byte) bool {
		subaccountID, denom := types.ParseDepositStoreKey(key)
		var deposit v2.Deposit
		k.cdc.MustUnmarshal(value, &deposit)
		return process(subaccountID, denom, &deposit)
	})
}
