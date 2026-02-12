package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func (k *BaseKeeper) SetOpenInterestForMarket(ctx sdk.Context, marketID common.Hash, openInterest math.LegacyDec) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DerivativeMarketOpenInterestPrefix)
	key := marketID.Bytes()

	if openInterest.IsZero() {
		store.Delete(key)
	} else {
		bz := types.UnsignedDecToUnsignedDecBytes(openInterest)
		store.Set(key, bz)
	}
}

func (k *BaseKeeper) ApplyOpenInterestDeltaForMarket(ctx sdk.Context, marketID common.Hash, openInterestDelta math.LegacyDec) {
	if openInterestDelta.IsZero() {
		return
	}

	currentOpenInterest := k.GetOpenInterestForMarket(ctx, marketID)
	newOpenInterest := currentOpenInterest.Add(openInterestDelta)

	// defensive programming: open interest should never be negative, but this avoids underflows
	if newOpenInterest.IsNegative() {
		newOpenInterest = math.LegacyZeroDec()
	}

	k.SetOpenInterestForMarket(ctx, marketID, newOpenInterest)
}

func (k *BaseKeeper) GetOpenInterestForMarket(ctx sdk.Context, marketID common.Hash) math.LegacyDec {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DerivativeMarketOpenInterestPrefix)
	key := marketID.Bytes()

	bz := store.Get(key)
	if bz == nil {
		return math.LegacyZeroDec()
	}

	openInterest := types.UnsignedDecBytesToDec(bz)
	return openInterest
}
