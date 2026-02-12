package binaryoptions

import (
	"github.com/InjectiveLabs/metrics"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/derivative"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/feediscounts"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/rewards"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/subaccount"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
)

//nolint:revive // ok
type BinaryOptionsKeeper struct {
	*base.BaseKeeper

	derivative   *derivative.DerivativeKeeper
	subaccount   *subaccount.SubaccountKeeper
	oracle       types.OracleKeeper
	account      authkeeper.AccountKeeper
	trading      *rewards.TradingKeeper
	feeDiscounts *feediscounts.FeeDiscountsKeeper

	svcTags metrics.Tags
}

func New(
	b *base.BaseKeeper,
	d *derivative.DerivativeKeeper,
	sk *subaccount.SubaccountKeeper,
	or types.OracleKeeper,
	ac authkeeper.AccountKeeper,
	tk *rewards.TradingKeeper,
	fd *feediscounts.FeeDiscountsKeeper,
) *BinaryOptionsKeeper {
	return &BinaryOptionsKeeper{
		BaseKeeper:   b,
		derivative:   d,
		oracle:       or,
		account:      ac,
		trading:      tk,
		feeDiscounts: fd,
		subaccount:   sk,
	}
}
