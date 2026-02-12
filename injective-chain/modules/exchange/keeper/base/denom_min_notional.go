package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) SetMinNotionalForDenom(ctx sdk.Context, denom string, minNotional math.LegacyDec) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DenomMinNotionalPrefix)
	key := []byte(denom)

	if minNotional.IsZero() {
		// If minNotional is zero, remove the entry from the store
		store.Delete(key)
	} else {
		// Otherwise, set the minNotional as before
		bz := types.UnsignedDecToUnsignedDecBytes(minNotional)
		store.Set(key, bz)
	}
}

func (k *BaseKeeper) GetMinNotionalForDenom(ctx sdk.Context, denom string) math.LegacyDec {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DenomMinNotionalPrefix)
	key := []byte(denom)

	bz := store.Get(key)
	if bz == nil {
		return math.LegacyZeroDec()
	}

	return types.UnsignedDecBytesToDec(bz)
}

func (k *BaseKeeper) HasMinNotionalForDenom(ctx sdk.Context, denom string) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DenomMinNotionalPrefix)
	key := []byte(denom)

	return store.Has(key)
}

func (k *BaseKeeper) GetAllDenomMinNotionals(ctx sdk.Context) []*v2.DenomMinNotional {
	minNotionals := make([]*v2.DenomMinNotional, 0)

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.DenomMinNotionalPrefix)

	iterateSafe(store.Iterator(nil, nil), func(key, value []byte) bool {
		denom := string(key)
		minNotional := types.UnsignedDecBytesToDec(value)
		minNotionals = append(minNotionals, &v2.DenomMinNotional{
			Denom:       denom,
			MinNotional: minNotional,
		})
		return false
	})

	return minNotionals
}
