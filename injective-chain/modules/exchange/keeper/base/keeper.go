package base

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

//nolint:revive // ok
type BaseKeeper struct {
	storeKey  storetypes.StoreKey
	tStoreKey storetypes.StoreKey
	cdc       codec.BinaryCodec
	svcTags   metrics.Tags
}

func NewBaseKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	tStoreKey storetypes.StoreKey,
) *BaseKeeper {
	return &BaseKeeper{
		storeKey:  storeKey,
		tStoreKey: tStoreKey,
		cdc:       cdc,
		svcTags: metrics.Tags{
			"svc": "exchange_base_k",
		},
	}
}

func (k *BaseKeeper) GetStoreKey() storetypes.StoreKey {
	return k.storeKey
}

func (k *BaseKeeper) GetCodec() codec.BinaryCodec {
	return k.cdc
}

func (k *BaseKeeper) getStore(ctx sdk.Context) storetypes.KVStore {
	return ctx.KVStore(k.storeKey)
}

func (k *BaseKeeper) getTransientStore(ctx sdk.Context) storetypes.KVStore {
	return ctx.TransientStore(k.tStoreKey)
}

// SetPostOnlyModeCancellationFlag sets a flag in the store to indicate that post-only mode
// should be cancelled in the next BeginBlock
func (k *BaseKeeper) SetPostOnlyModeCancellationFlag(ctx sdk.Context) {
	store := k.getStore(ctx)
	store.Set(types.PostOnlyModeCancellationKey, []byte{1})
}

// HasPostOnlyModeCancellationFlag checks if the post-only mode cancellation flag is set
func (k *BaseKeeper) HasPostOnlyModeCancellationFlag(ctx sdk.Context) bool {
	store := k.getStore(ctx)
	return store.Has(types.PostOnlyModeCancellationKey)
}

// DeletePostOnlyModeCancellationFlag removes the post-only mode cancellation flag from the store
func (k *BaseKeeper) DeletePostOnlyModeCancellationFlag(ctx sdk.Context) {
	store := k.getStore(ctx)
	store.Delete(types.PostOnlyModeCancellationKey)
}
