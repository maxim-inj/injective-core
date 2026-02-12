package orchestrator

import (
	"context"

	"github.com/InjectiveLabs/coretracer"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	peggytypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"
	"github.com/InjectiveLabs/injective-core/peggo/orchestrator/loops"
)

func (s *Orchestrator) runBatchCreator(ctx context.Context) (err error) {
	bc := batchCreator{
		Orchestrator: s,
		svcTags:      coretracer.NewTag("svc", "batch_creator"),
	}

	s.logger.WithField("loop_duration", s.cfg.LoopDuration.String()).Debugln("starting BatchCreator...")

	return loops.RunLoop(ctx, s.cfg.LoopDuration, func() error {
		return bc.requestTokenBatches(ctx)
	})
}

type batchCreator struct {
	*Orchestrator
	svcTags coretracer.Tags
}

func (l *batchCreator) Log() log.Logger {
	return l.logger.WithField("loop", "BatchCreator")
}

func (l *batchCreator) requestTokenBatches(ctx context.Context) error {
	defer coretracer.Trace(&ctx, l.svcTags)()

	fees, err := l.getUnbatchedTokenFees(ctx)
	if err != nil {
		coretracer.TraceError(ctx, err)
		l.Log().WithError(err).Warningln("failed to get withdrawal fees")
		return nil
	}

	if len(fees) == 0 {
		l.Log().Infoln("no withdrawals to batch")
		return nil
	}

	for _, fee := range fees {
		ok, err := l.checkFee(ctx, fee)
		if err != nil {
			l.Log().WithError(err).Warningln("error checking batch")
			continue
		}

		if !ok {
			continue
		}

		tokenAddr := gethcommon.HexToAddress(fee.Token)
		denom, ok := l.cfg.ERC20ContractMapping[tokenAddr]
		if !ok { // then it's a pegged asset
			denom = peggytypes.PeggyDenomString(tokenAddr)
		}

		if err := l.injective.SendRequestBatch(ctx, denom); err != nil {
			l.Log().WithError(err).Warningln("failed to request batch, perhaps it's already been requested?")
		}
	}

	return nil
}

func (l *batchCreator) getUnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	defer coretracer.Trace(&ctx, l.svcTags)()

	var fees []*peggytypes.BatchFees
	fn := func() (err error) {
		fees, err = l.injective.UnbatchedTokensWithFees(ctx)
		return
	}

	if err := l.retry(ctx, fn); err != nil {
		coretracer.TraceError(ctx, err)
		return nil, err
	}

	return fees, nil
}

func (l *batchCreator) checkFee(ctx context.Context, fee *peggytypes.BatchFees) (bool, error) {
	defer coretracer.Trace(&ctx, l.svcTags)()

	tokenAddress := gethcommon.HexToAddress(fee.Token)
	tokenDecimals, err := l.ethereum.TokenDecimals(ctx, tokenAddress)
	if err != nil {
		coretracer.TraceError(ctx, err)
		l.Log().WithError(err).Warningln("is token address valid?")
		return false, err
	}

	if l.cfg.MinBatchFeeUSD == 0 {
		return true, nil
	}

	tokenPriceUSDFloat, err := l.priceFeed.QueryUSDPrice(ctx, tokenAddress)
	if err != nil {
		coretracer.TraceError(ctx, err)
		l.Log().WithError(err).Warningln("failed to query price feed", "token_addr", tokenAddress.String())
		return false, err
	}

	var (
		minFeeUSD     = decimal.NewFromFloat(l.cfg.MinBatchFeeUSD)
		tokenPriceUSD = decimal.NewFromFloat(tokenPriceUSDFloat)
		totalFeeUSD   = decimal.NewFromBigInt(fee.TotalFees.BigInt(), -1*int32(tokenDecimals)).Mul(tokenPriceUSD)
	)

	l.Log().WithFields(log.Fields{
		"token_addr": fee.Token,
		"total_fee":  totalFeeUSD.String() + "USD",
		"min_fee":    minFeeUSD.String() + "USD",
	}).Debugln("checking batch fee")

	return totalFeeUSD.GreaterThanOrEqual(minFeeUSD), nil
}
