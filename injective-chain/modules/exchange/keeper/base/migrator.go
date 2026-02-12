package base

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/exported"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/migrations/v2"
)

type Migrator struct {
	keeper   BaseKeeper
	subspace exported.Subspace
}

func NewMigrator(k BaseKeeper, ss exported.Subspace) Migrator {
	return Migrator{
		keeper:   k,
		subspace: ss,
	}
}

func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	return v2.Migrate(
		ctx,
		ctx.KVStore(m.keeper.storeKey),
		m.subspace,
		m.keeper.cdc,
	)
}
