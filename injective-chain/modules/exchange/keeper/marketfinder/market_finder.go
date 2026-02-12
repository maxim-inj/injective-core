package marketfinder

import (
	"fmt"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

type CachedMarketFinder struct {
	*base.BaseKeeper
	spotMarkets          map[string]*v2.SpotMarket
	derivativeMarkets    map[string]*v2.DerivativeMarket
	binaryOptionsMarkets map[string]*v2.BinaryOptionsMarket
}

func New(k *base.BaseKeeper) *CachedMarketFinder {
	return &CachedMarketFinder{
		BaseKeeper:           k,
		spotMarkets:          make(map[string]*v2.SpotMarket),
		derivativeMarkets:    make(map[string]*v2.DerivativeMarket),
		binaryOptionsMarkets: make(map[string]*v2.BinaryOptionsMarket),
	}
}

func (mf *CachedMarketFinder) FindSpotMarket(ctx sdk.Context, marketId string) (*v2.SpotMarket, error) {
	market, found := mf.spotMarkets[marketId]
	if !found {
		market = mf.GetSpotMarketByID(ctx, common.HexToHash(marketId))
		if market == nil {
			return nil, errors.Wrapf(types.ErrSpotMarketNotFound, "spot market doesn't exist %s", marketId)
		}
		mf.spotMarkets[marketId] = market
	}

	return market, nil
}

func (mf *CachedMarketFinder) FindDerivativeMarket(ctx sdk.Context, marketId string) (*v2.DerivativeMarket, error) {
	market, found := mf.derivativeMarkets[marketId]
	if !found {
		market = mf.GetDerivativeMarketByID(ctx, common.HexToHash(marketId))
		if market == nil {
			return nil, errors.Wrapf(types.ErrDerivativeMarketNotFound, "derivative market for marketID %s not found", marketId)
		}
		mf.derivativeMarkets[marketId] = market
	}

	return market, nil
}

func (mf *CachedMarketFinder) FindBinaryOptionsMarket(ctx sdk.Context, marketId string) (*v2.BinaryOptionsMarket, error) {
	market, found := mf.binaryOptionsMarkets[marketId]
	if !found {
		market = mf.GetBinaryOptionsMarketByID(ctx, common.HexToHash(marketId))
		if market == nil {
			return nil, errors.Wrapf(types.ErrBinaryOptionsMarketNotFound, "binary options market for marketID %s not found", marketId)
		}
		mf.binaryOptionsMarkets[marketId] = market
	}

	return market, nil
}

func (mf *CachedMarketFinder) FindMarket(ctx sdk.Context, marketId string) (v2.MarketI, error) {
	var market v2.MarketI
	var err error
	var found bool

	market, found = mf.spotMarkets[marketId]
	if found {
		return market, nil
	}

	market, err = mf.FindSpotMarket(ctx, marketId)
	if err != nil {
		market, err = mf.FindDerivativeOrBinaryOptionsMarket(ctx, marketId)
	}

	return market, err
}

func (mf *CachedMarketFinder) FindDerivativeOrBinaryOptionsMarket(ctx sdk.Context, marketId string) (v2.DerivativeMarketI, error) {
	var market v2.DerivativeMarketI
	var err error
	var found bool

	market, found = mf.derivativeMarkets[marketId]
	if found {
		return market, nil
	}
	market, found = mf.binaryOptionsMarkets[marketId]
	if found {
		return market, nil
	}

	market, err = mf.FindDerivativeMarket(ctx, marketId)
	if err != nil {
		market, err = mf.FindBinaryOptionsMarket(ctx, marketId)
		if err != nil {
			return nil, fmt.Errorf("market for marketID %s not found", marketId)
		}
	}

	return market, nil
}
