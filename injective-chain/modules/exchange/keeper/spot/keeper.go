package spot

import (
	"github.com/InjectiveLabs/metrics"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/feediscounts"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/rewards"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/subaccount"
)

//nolint:revive // ok
type SpotKeeper struct {
	*base.BaseKeeper

	subaccount     *subaccount.SubaccountKeeper
	bank           bankkeeper.Keeper
	tradingRewards *rewards.TradingKeeper
	feeDiscounts   *feediscounts.FeeDiscountsKeeper

	svcTags metrics.Tags
}

func New(
	b *base.BaseKeeper,
	bk bankkeeper.Keeper,
	sa *subaccount.SubaccountKeeper,
	tk *rewards.TradingKeeper,
	fd *feediscounts.FeeDiscountsKeeper,
) *SpotKeeper {
	return &SpotKeeper{
		BaseKeeper:     b,
		bank:           bk,
		subaccount:     sa,
		tradingRewards: tk,
		feeDiscounts:   fd,

		svcTags: metrics.Tags{"svc": "spot_k"},
	}
}
