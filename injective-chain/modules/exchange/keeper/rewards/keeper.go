package rewards

import (
	"github.com/InjectiveLabs/metrics"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/feediscounts"
)

const (
	GROUPING_SECONDS_DEFAULT = 15
)

type TradingKeeper struct { //nolint:revive // ok
	*base.BaseKeeper

	feeDiscounts *feediscounts.FeeDiscountsKeeper
	bank         bankkeeper.Keeper
	distribution distrkeeper.Keeper

	svcTags metrics.Tags
}

func New(
	b *base.BaseKeeper,
	bk bankkeeper.Keeper,
	fd *feediscounts.FeeDiscountsKeeper,
	d distrkeeper.Keeper,
) *TradingKeeper {
	return &TradingKeeper{
		BaseKeeper:   b,
		bank:         bk,
		feeDiscounts: fd,
		distribution: d,
		svcTags:      metrics.Tags{"svc": "trading_k"},
	}
}
