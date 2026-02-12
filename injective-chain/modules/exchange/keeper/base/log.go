package base

import (
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

func (*BaseKeeper) Logger(ctx sdk.Context) log.Logger { // todo: sorry Abel, this was the fastest way forward
	return ctx.Logger().With("module", types.ModuleName)
}
