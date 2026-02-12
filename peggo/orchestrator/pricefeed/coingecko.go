package pricefeed

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/InjectiveLabs/coretracer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"
)

const (
	maxRespTime        = 15 * time.Second
	maxRespHeadersTime = 15 * time.Second
	maxRespBytes       = 10 * 1024 * 1024
)

var zeroPrice = float64(0)

type Config struct {
	BaseURL string
}

type CoingeckoPriceFeed struct {
	client *http.Client
	config *Config

	interval time.Duration

	logger  log.Logger
	svcTags coretracer.Tags
}

// NewCoingeckoPriceFeed returns price puller for given symbol. The price will be pulled
// from endpoint and divided by scaleFactor. Symbol name (if reported by endpoint) must match.
func NewCoingeckoPriceFeed(interval time.Duration, endpointConfig *Config) *CoingeckoPriceFeed {
	return &CoingeckoPriceFeed{
		client: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: maxRespHeadersTime,
			},
			Timeout: maxRespTime,
		},
		config: checkCoingeckoConfig(endpointConfig),

		interval: interval,

		logger: log.WithFields(log.Fields{
			"svc":      "oracle",
			"provider": "coingeckgo",
		}),
		svcTags: coretracer.NewTag("oracle_provider", "coingeckgo"),
	}
}

func urlJoin(baseURL string, segments ...string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	u.Path = path.Join(append([]string{u.Path}, segments...)...)
	return u.String()

}

func (cp *CoingeckoPriceFeed) QueryUSDPrice(ctx context.Context, erc20Contract common.Address) (float64, error) {
	defer coretracer.Trace(&ctx, cp.svcTags)()

	u, err := url.ParseRequestURI(urlJoin(cp.config.BaseURL, "simple", "token_price", "ethereum"))
	if err != nil {
		coretracer.TraceError(ctx, err)
		cp.logger.WithError(err).Fatalln("failed to parse URL")
	}

	q := make(url.Values)

	q.Set("contract_addresses", strings.ToLower(erc20Contract.String()))
	q.Set("vs_currencies", "usd")
	u.RawQuery = q.Encode()

	reqURL := u.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		coretracer.TraceError(ctx, err)
		cp.logger.WithError(err).Fatalln("failed to create HTTP request")
		return zeroPrice, err
	}

	resp, err := cp.client.Do(req)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return zeroPrice, errors.Wrapf(err, "failed to fetch price from %s", reqURL)
	}

	respBody, err := ioutil.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		_ = resp.Body.Close()
		coretracer.TraceError(ctx, err)
		return zeroPrice, errors.Wrapf(err, "failed to read response body from %s", reqURL)
	}

	_ = resp.Body.Close()

	var f interface{}
	if err := json.Unmarshal(respBody, &f); err != nil {
		coretracer.TraceError(ctx, err)
		return zeroPrice, err
	}

	m, ok := f.(map[string]interface{})
	if !ok {
		err = errors.Errorf("failed to cast response type: map[string]interface{}")
		coretracer.TraceError(ctx, err)
		return zeroPrice, err
	}

	v := m[strings.ToLower(erc20Contract.String())]
	if v == nil {
		err = errors.Errorf("failed to get contract address")
		coretracer.TraceError(ctx, err)
		return zeroPrice, err
	}

	n, ok := v.(map[string]interface{})
	if !ok {
		err = errors.Errorf("failed to cast value type: map[string]interface{}")
		coretracer.TraceError(ctx, err)
		return zeroPrice, err
	}

	tokenPriceInUSD := n["usd"].(float64)
	return tokenPriceInUSD, nil
}

func checkCoingeckoConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}

	if len(cfg.BaseURL) == 0 {
		cfg.BaseURL = "https://api.coingecko.com/api/v3"
	}

	return cfg
}

func (cp *CoingeckoPriceFeed) CheckFeeThreshold(
	ctx context.Context,
	erc20Contract common.Address,
	totalFee sdkmath.Int,
	minFeeInUSD float64,
) bool {
	defer coretracer.Trace(&ctx, cp.svcTags)()

	tokenPriceInUSD, err := cp.QueryUSDPrice(ctx, erc20Contract)
	if err != nil {
		coretracer.TraceError(ctx, err)
		return false
	}

	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(totalFee.BigInt(), -18).Mul(tokenPriceInUSDDec)
	minFeeInUSDDec := decimal.NewFromFloat(minFeeInUSD)

	if totalFeeInUSDDec.GreaterThan(minFeeInUSDDec) {
		return true
	}
	return false
}
