// iavl-key-compare compares IAVL tree key-value pairs between a local database and a canonical RPC node.
// This tool is useful for debugging app hash mismatches by identifying which specific keys diverged.
//
// The tool opens the local LevelDB database directly and iterates through all keys in the specified
// store's IAVL tree. For each key, it queries the canonical node via RPC to compare values. Keys are
// grouped by their first byte (prefix) to enable efficient comparison and reporting.
//
// For divergent keys, the tool provides detailed analysis including:
//   - Decoded key structure (e.g., market ID, direction, price for orderbook levels)
//   - Value comparison with quantity decoding (assumes 18 decimal places for Dec types)
//   - Ratio calculation between local and canonical values
//   - Historical comparison showing the key's state at heights N-2, N-1, and N
//
// NB: The local node must be stopped before running this tool to avoid database lock conflicts.
//
// # Flags
//
//	-db          Path to the directory containing application.db (required)
//	-canonical   Canonical node RPC URL (required)
//	-height      Block height to compare (required)
//	-store       Store name to compare (default: "exchange")
//	-workers     Number of parallel workers for RPC queries (default: 50)
//
// # Example
//
//	iavl-key-compare -db /data -canonical http://archival:26657 -height 144213864 -store exchange
//
// # Exit Codes
//
//	0 - All keys match
//	1 - Divergence detected or error occurred
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/iavl"

	"cosmossdk.io/store/wrapper"
)

// exchangePrefixNames contains known key prefixes for the exchange module.
var exchangePrefixNames = map[byte]string{
	0x01: "Deposits",
	0x02: "Positions",
	0x03: "SubaccountOrders",
	0x04: "SpotMarket",
	0x07: "SpotMarketParamUpdate",
	0x08: "DerivativeMarketParamUpdate",
	0x09: "SpotMarketScheduledSettlementInfo",
	0x0a: "DerivativeMarketScheduledSettlementInfo",
	0x0b: "SubaccountTradeNonce",
	0x0c: "SpotLimitOrderIndicator",
	0x0d: "DerivativeOrderbookMidPriceAndTOBPrice",
	0x0e: "ExpiryFuturesMarketInfo",
	0x0f: "PerpetualMarketFunding",
	0x10: "PerpetualMarketInfo",
	0x11: "SubaccountMarketOrder",
	0x12: "SpotOrdersByMarketDirectionPriceOrderHash",
	0x14: "DerivativeOrdersByMarketDirectionPriceOrderHash",
	0x18: "SpotOrderbookLevels",
	0x21: "DerivativeMarket",
	0x22: "DerivativeLimitOrders",
	0x24: "DerivativeLimitOrderIndicator",
	0x27: "ConditionalDerivativeOrders",
	0x2b: "DerivativeOrderbookLevels",
	0x31: "BinaryOptionsMarket",
	0x32: "BinaryOptionsLimitOrders",
	0x33: "BinaryOptionsMarketParamUpdate",
	0x34: "FeeDiscountSchedule",
	0x35: "FeeDiscountAccountTierInfo",
	0x45: "IsFirstFeeCycleFinished",
	0x46: "TradingRewardCampaignInfo",
	0x50: "TradingRewardAccountPoints",
	0x51: "TradingRewardPoolCampaignSchedule",
	0x52: "TradingRewardPoolAccountPendingRewardsSchedule",
	0x53: "TradingRewardCampaignAccountPendingRewardsSchedule",
	0x54: "SpotOrderbookMidPriceAndTOBPrice",
	0x56: "SubaccountCid",
	0x57: "DenomDecimals",
	0x58: "GrantAuthorisations",
	0x60: "GranteeLastGrantDelegatedAmount",
	0x61: "ActiveGrant",
}

type prefixData struct {
	Prefix    byte
	Keys      [][]byte
	Values    [][]byte
	LocalHash []byte
}

type abciQueryResponse struct {
	Result struct {
		Response struct {
			Code  int    `json:"code"`
			Value string `json:"value"`
		} `json:"response"`
	} `json:"result"`
}

type divergentKey struct {
	Key            []byte
	LocalValue     []byte
	CanonicalValue []byte
}

func main() {
	var (
		dbPath       string
		canonicalURL string
		height       int64
		storeName    string
		workers      int
	)

	flag.StringVar(&dbPath, "db", "", "Path to local application.db directory (required)")
	flag.StringVar(&canonicalURL, "canonical", "", "Canonical node RPC URL (required)")
	flag.Int64Var(&height, "height", 0, "Block height to compare (required)")
	flag.StringVar(&storeName, "store", "exchange", "Store name to compare")
	flag.IntVar(&workers, "workers", 50, "Number of parallel workers for RPC queries")
	flag.Parse()

	if dbPath == "" || canonicalURL == "" || height == 0 {
		fmt.Fprintln(os.Stderr, "Error: -db, -canonical, and -height are required")
		fmt.Fprintln(os.Stderr, "Usage: iavl-key-compare -db <path> -canonical <url> -height <height> [-store <name>] [-workers <n>]")
		os.Exit(1)
	}

	fmt.Printf("Opening database: %s\n", dbPath)
	fmt.Printf("Canonical node: %s\n", canonicalURL)
	fmt.Printf("Height: %d\n", height)
	fmt.Printf("Store: %s\n", storeName)
	fmt.Println()

	db, err := dbm.NewGoLevelDB("application", dbPath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	prefix := []byte("s/k:" + storeName + "/")
	prefixDB := dbm.NewPrefixDB(db, prefix)
	wrappedDB := wrapper.NewDBWrapper(prefixDB)

	tree := iavl.NewMutableTree(wrappedDB, 10000, false, iavl.NewNopLogger())
	version, err := tree.LoadVersion(height)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load version %d: %v\n", height, err)
		os.Exit(1)
	}
	fmt.Printf("Loaded IAVL tree at version: %d\n", version)

	immutable, err := tree.GetImmutable(height)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get immutable tree: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nPhase 1: Iterating local IAVL tree...")
	prefixes := make(map[byte]*prefixData)
	keyCount := 0

	immutable.Iterate(func(key, value []byte) bool {
		if len(key) == 0 {
			return false
		}
		p := key[0]
		if prefixes[p] == nil {
			prefixes[p] = &prefixData{Prefix: p}
		}
		keyCopy := make([]byte, len(key))
		valueCopy := make([]byte, len(value))
		copy(keyCopy, key)
		copy(valueCopy, value)
		prefixes[p].Keys = append(prefixes[p].Keys, keyCopy)
		prefixes[p].Values = append(prefixes[p].Values, valueCopy)
		keyCount++
		if keyCount%10000 == 0 {
			fmt.Printf("  Processed %d keys...\n", keyCount)
		}
		return false
	})

	fmt.Printf("Total keys: %d across %d prefixes\n", keyCount, len(prefixes))

	fmt.Println("\nPhase 2: Computing local prefix hashes...")
	for p, data := range prefixes {
		h := sha256.New()
		for i := range data.Keys {
			h.Write(data.Keys[i])
			h.Write(data.Values[i])
		}
		data.LocalHash = h.Sum(nil)
		name := getPrefixName(storeName, p)
		fmt.Printf("  Prefix 0x%02x (%s): %d keys, hash=%s\n", p, name, len(data.Keys), hex.EncodeToString(data.LocalHash)[:16])
	}

	fmt.Println("\nPhase 3: Querying canonical node for comparison...")

	var allDivergentKeys []divergentKey
	var divergentPrefixes []byte

	sortedPrefixes := make([]byte, 0, len(prefixes))
	for p := range prefixes {
		sortedPrefixes = append(sortedPrefixes, p)
	}
	sort.Slice(sortedPrefixes, func(i, j int) bool { return sortedPrefixes[i] < sortedPrefixes[j] })

	for _, p := range sortedPrefixes {
		data := prefixes[p]
		name := getPrefixName(storeName, p)
		fmt.Printf("\nChecking prefix 0x%02x (%s) - %d keys...\n", p, name, len(data.Keys))

		canonicalValues := make([][]byte, len(data.Keys))
		var queried int64
		var errors int64

		jobs := make(chan int, len(data.Keys))
		var wg sync.WaitGroup

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range jobs {
					value, err := queryCanonicalKey(canonicalURL, storeName, data.Keys[idx], height)
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						canonicalValues[idx] = value
					}
					count := atomic.AddInt64(&queried, 1)
					if count%1000 == 0 {
						fmt.Printf("  Queried %d/%d keys...\n", count, len(data.Keys))
					}
				}
			}()
		}

		for i := range data.Keys {
			jobs <- i
		}
		close(jobs)
		wg.Wait()

		if errors > 0 {
			fmt.Printf("  Warning: %d query errors\n", errors)
		}

		h := sha256.New()
		for i := range data.Keys {
			h.Write(data.Keys[i])
			h.Write(canonicalValues[i])
		}
		canonicalHash := h.Sum(nil)

		localHashStr := hex.EncodeToString(data.LocalHash)[:16]
		canonicalHashStr := hex.EncodeToString(canonicalHash)[:16]

		if localHashStr != canonicalHashStr {
			fmt.Printf("  DIVERGENT: local=%s canonical=%s\n", localHashStr, canonicalHashStr)
			divergentPrefixes = append(divergentPrefixes, p)

			fmt.Println("  Finding divergent keys...")
			divergentCount := 0
			for i := range data.Keys {
				if string(data.Values[i]) != string(canonicalValues[i]) {
					divergentCount++
					allDivergentKeys = append(allDivergentKeys, divergentKey{
						Key:            data.Keys[i],
						LocalValue:     data.Values[i],
						CanonicalValue: canonicalValues[i],
					})
					if divergentCount <= 10 {
						fmt.Printf("    Key: %s\n", hex.EncodeToString(data.Keys[i]))
						fmt.Printf("      Local:     %s\n", truncateHex(data.Values[i], 50))
						fmt.Printf("      Canonical: %s\n", truncateHex(canonicalValues[i], 50))
					}
				}
			}
			fmt.Printf("  Total divergent keys in prefix: %d\n", divergentCount)
		} else {
			fmt.Printf("  MATCH\n")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	if len(divergentPrefixes) == 0 {
		fmt.Println("All prefixes match!")
	} else {
		fmt.Printf("Divergent prefixes: ")
		for _, p := range divergentPrefixes {
			name := getPrefixName(storeName, p)
			fmt.Printf("0x%02x (%s) ", p, name)
		}
		fmt.Println()

		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println("DETAILED DIVERGENCE ANALYSIS")
		fmt.Println(strings.Repeat("=", 80))

		for i, dk := range allDivergentKeys {
			fmt.Printf("\n--- Divergent Key #%d ---\n", i+1)
			analyseKey(dk, tree, height, canonicalURL, storeName)
		}

		os.Exit(1)
	}
}

func getPrefixName(storeName string, prefix byte) string {
	if storeName == "exchange" {
		if name, ok := exchangePrefixNames[prefix]; ok {
			return name
		}
	}
	return "Unknown"
}

func analyseKey(dk divergentKey, tree *iavl.MutableTree, height int64, canonicalURL, storeName string) {
	key := dk.Key
	if len(key) == 0 {
		return
	}

	prefix := key[0]
	prefixName := getPrefixName(storeName, prefix)

	fmt.Printf("Prefix: 0x%02x (%s)\n", prefix, prefixName)
	fmt.Printf("Key (hex): %s\n", hex.EncodeToString(key))

	if prefix == 0x2b || prefix == 0x18 {
		parseOrderbookLevelKey(key)
	}

	localQty := decodeQuantity(dk.LocalValue)
	canonicalQty := decodeQuantity(dk.CanonicalValue)

	fmt.Printf("\nValues:\n")
	fmt.Printf("  Local value (hex):     %s\n", hex.EncodeToString(dk.LocalValue))
	fmt.Printf("  Canonical value (hex): %s\n", hex.EncodeToString(dk.CanonicalValue))
	fmt.Printf("  Local quantity:        %s\n", localQty)
	fmt.Printf("  Canonical quantity:    %s\n", canonicalQty)

	if canonicalQty != "0" && canonicalQty != "" {
		localF, _ := new(big.Float).SetString(localQty)
		canonicalF, _ := new(big.Float).SetString(canonicalQty)
		if localF != nil && canonicalF != nil && canonicalF.Sign() != 0 {
			ratio := new(big.Float).Quo(localF, canonicalF)
			fmt.Printf("  Ratio (local/canonical): %s\n", ratio.Text('f', 6))
		}
	}

	fmt.Printf("\nLocal IAVL History:\n")
	fmt.Printf("%-12s | %-20s | %-20s | %s\n", "Height", "Local", "Canonical", "Status")
	fmt.Printf("%s\n", strings.Repeat("-", 80))

	for delta := int64(2); delta >= 0; delta-- {
		checkHeight := height - delta
		localVal := queryLocalIAVL(tree, key, checkHeight)
		canonicalVal, _ := queryCanonicalKey(canonicalURL, storeName, key, checkHeight)

		localStr := formatValue(localVal)
		canonicalStr := formatValue(canonicalVal)

		status := "Match"
		if localStr != canonicalStr {
			status = "DIVERGED"
		}

		fmt.Printf("%-12d | %-20s | %-20s | %s\n", checkHeight, localStr, canonicalStr, status)
	}
}

func parseOrderbookLevelKey(key []byte) {
	if len(key) < 34 {
		return
	}

	marketID := key[1:33]
	direction := key[33]
	priceBytes := key[34:]

	dirStr := "SELL"
	if direction == 0x01 {
		dirStr = "BUY"
	}

	priceStr := string(priceBytes)
	priceStr = strings.TrimLeft(priceStr, "0")
	if priceStr == "" || priceStr[0] == '.' {
		priceStr = "0" + priceStr
	}

	fmt.Printf("Market ID: 0x%s\n", hex.EncodeToString(marketID))
	fmt.Printf("Direction: %s\n", dirStr)
	fmt.Printf("Price: %s\n", priceStr)
}

func decodeQuantity(data []byte) string {
	if len(data) == 0 {
		return "0"
	}

	val := new(big.Int).SetBytes(data)
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	valFloat := new(big.Float).SetInt(val)
	divisorFloat := new(big.Float).SetInt(divisor)
	result := new(big.Float).Quo(valFloat, divisorFloat)

	return result.Text('f', 6)
}

func formatValue(data []byte) string {
	if len(data) == 0 {
		return "Key not found"
	}
	return decodeQuantity(data)
}

func queryLocalIAVL(tree *iavl.MutableTree, key []byte, height int64) []byte {
	immutable, err := tree.GetImmutable(height)
	if err != nil {
		return nil
	}
	val, err := immutable.Get(key)
	if err != nil {
		return nil
	}
	return val
}

func queryCanonicalKey(nodeURL, storeName string, key []byte, height int64) ([]byte, error) {
	keyHex := hex.EncodeToString(key)
	path := fmt.Sprintf(`"/store/%s/key"`, storeName)
	queryURL := fmt.Sprintf("%s/abci_query?path=%s&data=0x%s&height=%d",
		nodeURL, url.QueryEscape(path), keyHex, height)

	resp, err := http.Get(queryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result abciQueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Result.Response.Value == "" {
		return nil, nil
	}

	value, err := base64.StdEncoding.DecodeString(result.Result.Response.Value)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func truncateHex(data []byte, maxLen int) string {
	s := hex.EncodeToString(data)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
