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

func (k *BaseKeeper) GetMarketBalance(ctx sdk.Context, marketID common.Hash) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetDerivativeMarketBalanceKey(marketID)

	bz := store.Get(key)
	if bz == nil {
		return math.LegacyZeroDec()
	}
	return types.SignedDecBytesToDec(bz)
}

func (k *BaseKeeper) GetAllMarketBalances(ctx sdk.Context) []*v2.MarketBalance {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	balances := make([]*v2.MarketBalance, 0)

	marketBalancesStore := prefix.NewStore(store, types.MarketBalanceKey)

	iterateSafe(marketBalancesStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key).String()
		balance := types.SignedDecBytesToDec(value)
		balances = append(balances, &v2.MarketBalance{
			MarketId: marketID,
			Balance:  balance,
		})
		return false
	})

	return balances
}

func (k *BaseKeeper) SetMarketBalance(ctx sdk.Context, marketID common.Hash, balance math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if balance.IsNil() || balance.IsZero() {
		k.DeleteMarketBalance(ctx, marketID)
		return
	}

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.MarketBalanceKey)
	store.Set(marketID.Bytes(), types.SignedDecToSignedDecBytes(balance))
}

func (k *BaseKeeper) DeleteMarketBalance(
	ctx sdk.Context,
	marketID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.MarketBalanceKey)
	store.Delete(marketID.Bytes())
}
