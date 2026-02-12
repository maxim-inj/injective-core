// compare-store-hashes compares IAVL store root hashes between two nodes at a specific height.
// This tool is useful for debugging app hash mismatches by identifying which store(s) diverged.
//
// The tool queries both nodes via their Tendermint RPC /abci_query endpoint with prove=true.
// This returns an ICS23 commitment proof for the queried key. The proof contains the Merkle
// path from the key to the store's root hash, allowing us to extract the root hash without
// needing to iterate the entire store.
//
// For each of the 37 Cosmos SDK and Injective module stores, the tool:
//  1. Tries known key prefixes that are likely to contain data (e.g., 0x02 for bank balances)
//  2. Falls back to generic keys if module-specific prefixes fail
//  3. Extracts the IAVL root hash from the ICS23 existence or non-existence proof
//  4. Compares the root hashes between the two nodes
//
// Stores that have no data or cannot be queried are marked as "(empty)" or "(unqueryable)".
//
// # Usage
//
//	compare-store-hashes -height <height> -canonical <url> [-local <url>] [-diff]
//
// # Flags
//
//	-height      Block height to query (required)
//	-canonical   Canonical/archival node RPC URL (required)
//	-local       Local node RPC URL (default: http://localhost:26657)
//	-diff        Show only divergent store hashes (default: false)
//
// # Example
//
//	compare-store-hashes -height 144213864 -canonical http://archival:26657 -local http://localhost:26657 -diff
//
// # Exit Codes
//
//	0 - All stores match
//	1 - Divergence detected in one or more stores
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	ics23 "github.com/cosmos/ics23/go"
)

// stores is the list of all Cosmos SDK and Injective module stores to query.
var stores = []string{
	"acc", "auction", "authz", "bank", "capability", "chainlink", "consensus",
	"crisis", "distribution", "downtimedetector", "erc20", "evidence", "evm",
	"exchange", "feegrant", "feeibc", "gov", "hooks-for-ibc", "hyperlane",
	"ibc", "icahost", "insurance", "mint", "oracle", "packetfowardmiddleware",
	"params", "peggy", "permissions", "slashing", "staking", "tokenfactory",
	"transfer", "txfees", "upgrade", "warp", "wasm", "xwasm",
}

// moduleKeyPrefixes contains known key prefixes for various modules that are likely to have data.
// These are tried first when querying for store proofs.
var moduleKeyPrefixes = map[string][][]byte{
	"staking":      {{0x21}, {0x11}, {0x01}},
	"bank":         {{0x02}, {0x01}},
	"distribution": {{0x02}, {0x01}},
	"gov":          {{0x00}, {0x01}},
	"slashing":     {{0x01}, {0x00}},
	"mint":         {{0x00}, {0x01}},
	"auth":         {{0x01}},
	"acc":          {{0x01}},
	"authz":        {{0x01}},
	"feegrant":     {{0x00}},
	"ibc":          {{0x00}},
	"transfer":     {{0x00}},
	"capability":   {{0x00}},
	"params":       {{0x00}},
	"upgrade":      {{0x00}},
	"evidence":     {{0x00}},
	"crisis":       {{0x00}},
}

type abciQueryResponse struct {
	Result struct {
		Response struct {
			Code      int       `json:"code"`
			Log       string    `json:"log"`
			ProofOps  *proofOps `json:"proofOps"`
			Height    string    `json:"height"`
			Codespace string    `json:"codespace"`
		} `json:"response"`
	} `json:"result"`
}

type proofOps struct {
	Ops []proofOp `json:"ops"`
}

type proofOp struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	Data string `json:"data"`
}

type storeHash struct {
	Hash []byte
	Err  error
}

func main() {
	var (
		canonicalURL string
		localURL     string
		height       int64
		diffOnly     bool
	)

	flag.StringVar(&canonicalURL, "canonical", "", "Canonical/archival node RPC URL (required)")
	flag.StringVar(&localURL, "local", "http://localhost:26657", "Local node RPC URL")
	flag.Int64Var(&height, "height", 0, "Block height to query (required)")
	flag.BoolVar(&diffOnly, "diff", false, "Show only divergent store hashes")
	flag.Parse()

	if height == 0 || canonicalURL == "" {
		fmt.Fprintln(os.Stderr, "Error: -height and -canonical are required")
		fmt.Fprintln(os.Stderr, "Usage: compare-store-hashes -height <height> -canonical <url> [-local <url>] [-diff]")
		os.Exit(1)
	}

	fmt.Printf("Comparing store hashes at height %d\n", height)
	fmt.Printf("Canonical: %s\n", canonicalURL)
	fmt.Printf("Local:     %s\n", localURL)
	fmt.Println()

	canonicalHashes := queryAllStores(canonicalURL, height)
	localHashes := queryAllStores(localURL, height)

	sortedStores := make([]string, len(stores))
	copy(sortedStores, stores)
	sort.Strings(sortedStores)

	storeColWidth := 25
	hashColWidth := 66

	separator := strings.Repeat("-", storeColWidth+hashColWidth*2+7)
	fmt.Println(separator)
	fmt.Printf("| %-*s | %-*s | %-*s |\n",
		storeColWidth, "Store",
		hashColWidth, "Canonical Node",
		hashColWidth, "Local Node")
	fmt.Println(separator)

	var matchingHashes, unqueryable, divergent, oneError int

	for _, store := range sortedStores {
		canonical := canonicalHashes[store]
		local := localHashes[store]

		canonicalStr := formatHash(canonical)
		localStr := formatHash(local)

		match := hashesMatch(canonical, local)

		if match {
			if canonical.Err != nil {
				unqueryable++
			} else {
				matchingHashes++
			}
		} else if canonical.Err != nil || local.Err != nil {
			oneError++
		} else {
			divergent++
		}

		if diffOnly && match {
			continue
		}

		marker := ""
		if !match {
			if canonical.Err == nil && local.Err == nil {
				marker = " DIVERGENT"
			} else {
				marker = " ONE FAILED"
			}
		}

		fmt.Printf("| %-*s | %-*s | %-*s |%s\n",
			storeColWidth, store,
			hashColWidth, canonicalStr,
			hashColWidth, localStr,
			marker)
	}

	fmt.Println(separator)
	fmt.Println()
	fmt.Printf("Summary: %d matching, %d unqueryable (both), %d divergent, %d one-side errors\n",
		matchingHashes, unqueryable, divergent, oneError)

	if divergent > 0 {
		fmt.Println("\nDIVERGENCE DETECTED!")
		os.Exit(1)
	} else if oneError > 0 {
		fmt.Println("\nSome stores could only be queried from one node")
	} else if unqueryable > 0 {
		fmt.Println("\nAll queryable stores match! (some stores unqueryable on both nodes)")
	} else {
		fmt.Println("\nAll stores match!")
	}
}

func queryAllStores(nodeURL string, height int64) map[string]storeHash {
	results := make(map[string]storeHash)
	for _, store := range stores {
		hash, err := queryStoreHash(nodeURL, store, height)
		results[store] = storeHash{Hash: hash, Err: err}
	}
	return results
}

func queryStoreHash(nodeURL, storeName string, height int64) ([]byte, error) {
	if prefixes, ok := moduleKeyPrefixes[storeName]; ok {
		for _, prefix := range prefixes {
			hash, err := queryStoreHashWithKey(nodeURL, storeName, height, prefix)
			if err == nil {
				return hash, nil
			}
		}
	}

	keysToTry := [][]byte{
		{0x00},
		{0x01},
		{0x02},
		[]byte("a"),
		[]byte("dummy_key_for_proof"),
		{0xff, 0xff, 0xff, 0xff},
	}

	var lastErr error
	for _, key := range keysToTry {
		hash, err := queryStoreHashWithKey(nodeURL, storeName, height, key)
		if err == nil {
			return hash, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

func queryStoreHashWithKey(nodeURL, storeName string, height int64, key []byte) ([]byte, error) {
	keyHex := hex.EncodeToString(key)
	path := fmt.Sprintf(`"/store/%s/key"`, storeName)
	queryURL := fmt.Sprintf("%s/abci_query?path=%s&data=0x%s&height=%d&prove=true",
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

	if result.Result.Response.ProofOps == nil || len(result.Result.Response.ProofOps.Ops) == 0 {
		return nil, fmt.Errorf("no proof returned")
	}

	for _, op := range result.Result.Response.ProofOps.Ops {
		if op.Type == "ics23:iavl" {
			hash, err := extractRootFromProof(op.Data)
			if err == nil {
				return hash, nil
			}
			return nil, err
		}
	}

	return nil, fmt.Errorf("no IAVL proof found")
}

func extractRootFromProof(proofDataB64 string) ([]byte, error) {
	proofData, err := base64.StdEncoding.DecodeString(proofDataB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode proof: %w", err)
	}

	var commitProof ics23.CommitmentProof
	if err := commitProof.Unmarshal(proofData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal commitment proof: %w", err)
	}

	if root, err := commitProof.Calculate(); err == nil {
		return root, nil
	}

	if exist := commitProof.GetExist(); exist != nil {
		if root, err := exist.Calculate(); err == nil {
			return root, nil
		}
	}

	if nonExist := commitProof.GetNonexist(); nonExist != nil {
		if nonExist.Left == nil && nonExist.Right == nil {
			return nil, fmt.Errorf("empty store")
		}

		if nonExist.Right != nil && len(nonExist.Right.Value) > 0 {
			if root, err := nonExist.Right.Calculate(); err == nil {
				return root, nil
			}
		}
		if nonExist.Left != nil && len(nonExist.Left.Value) > 0 {
			if root, err := nonExist.Left.Calculate(); err == nil {
				return root, nil
			}
		}

		return nil, fmt.Errorf("neighbours have no values")
	}

	if batch := commitProof.GetBatch(); batch != nil {
		for _, entry := range batch.Entries {
			if exist := entry.GetExist(); exist != nil {
				if root, err := exist.Calculate(); err == nil {
					return root, nil
				}
			}
			if nonExist := entry.GetNonexist(); nonExist != nil {
				if nonExist.Right != nil && len(nonExist.Right.Value) > 0 {
					if root, err := nonExist.Right.Calculate(); err == nil {
						return root, nil
					}
				}
				if nonExist.Left != nil && len(nonExist.Left.Value) > 0 {
					if root, err := nonExist.Left.Calculate(); err == nil {
						return root, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("failed to extract root from any proof type")
}

func formatHash(sh storeHash) string {
	if sh.Err != nil {
		errMsg := sh.Err.Error()
		if strings.Contains(errMsg, "neighbours have no values") {
			return "(unqueryable)"
		}
		if strings.Contains(errMsg, "empty store") {
			return "(empty)"
		}
		if strings.Contains(errMsg, "failed to extract root") {
			return "(unqueryable)"
		}
		return fmt.Sprintf("ERR: %s", truncate(errMsg, 40))
	}
	if sh.Hash == nil {
		return "N/A"
	}
	return fmt.Sprintf("%X", sh.Hash)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func hashesMatch(a, b storeHash) bool {
	if a.Err != nil && b.Err != nil {
		return true
	}
	if a.Err != nil || b.Err != nil {
		return false
	}
	return bytes.Equal(a.Hash, b.Hash)
}
