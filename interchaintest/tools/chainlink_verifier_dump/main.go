package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

const eip1967ImplementationSlot = "0x360894A13BA1A3210667C828492DB98DCA3E2076CC3735A920A3CA505D382BBC"
const verifierMappingSlot = 3

type storageEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type contractDump struct {
	Address string         `json:"address"`
	Code    string         `json:"code"`
	Storage []storageEntry `json:"storage"`
}

type dumpMeta struct {
	RPCAddress            string `json:"rpc_address"`
	BlockTag              string `json:"block_tag"`
	BlockHash             string `json:"block_hash"`
	TxIndex               uint64 `json:"tx_index"`
	ImplementationSlot    string `json:"implementation_slot"`
	ImplementationFound   bool   `json:"implementation_found"`
	VerifierMappingSource string `json:"verifier_mapping_source"`
}

type verifierDump struct {
	Proxy            contractDump      `json:"proxy"`
	Implementation   *contractDump     `json:"implementation,omitempty"`
	VerifierMappings []verifierMapping `json:"verifier_mappings,omitempty"`
	Verifiers        []contractDump    `json:"verifiers,omitempty"`
	Meta             dumpMeta          `json:"meta"`
}

type blockHeader struct {
	Hash common.Hash `json:"hash"`
}

type storageRangeEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type storageRangeResult struct {
	Storage map[string]storageRangeEntry `json:"storage"`
	NextKey string                       `json:"nextKey"`
}

type configDigestList []common.Hash

func (c *configDigestList) String() string {
	if c == nil {
		return ""
	}
	parts := make([]string, 0, len(*c))
	for _, digest := range *c {
		parts = append(parts, digest.Hex())
	}
	return strings.Join(parts, ",")
}

func (c *configDigestList) Set(value string) error {
	digest, err := parseConfigDigest(value)
	if err != nil {
		return err
	}
	*c = append(*c, digest)
	return nil
}

type verifierMapping struct {
	ConfigDigest string `json:"config_digest,omitempty"`
	StorageKey   string `json:"storage_key"`
	Verifier     string `json:"verifier"`
}

const verifierInterfaceSignature = "verify(bytes,address)"

func main() {
	var (
		rpcAddr       string
		address       string
		blockTag      string
		maxResults    uint64
		outPath       string
		timeout       time.Duration
		configDigests configDigestList
	)

	defaultRPC := os.Getenv("ETH_RPC_URL")
	if defaultRPC == "" {
		defaultRPC = "http://localhost:8545"
	}
	flag.StringVar(&rpcAddr, "rpc", defaultRPC, "Ethereum JSON-RPC endpoint (env: ETH_RPC_URL)")
	flag.StringVar(&address, "address", "0x60fAa7faC949aF392DFc858F5d97E3EEfa07E9EB", "Verifier proxy contract address")
	flag.StringVar(&blockTag, "block", "latest", "Block tag (e.g. latest or hex block number)")
	flag.Uint64Var(&maxResults, "max-results", 1000, "Maximum results per debug_storageRangeAt call")
	flag.StringVar(&outPath, "out", "", "Optional output path (defaults to stdout)")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "RPC timeout")
	flag.Var(&configDigests, "config-digest", "Config digest (32-byte hash). Can be repeated.")
	flag.Parse()

	if rpcAddr == "" {
		log.Fatal("missing RPC endpoint (use --rpc or ETH_RPC_URL)")
	}
	if !common.IsHexAddress(address) {
		log.Fatalf("invalid contract address: %s", address)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := rpc.DialContext(ctx, rpcAddr)
	if err != nil {
		log.Fatalf("failed to connect to RPC: %v", err)
	}
	defer client.Close()

	blockHash, txIndex, err := resolveBlock(ctx, client, blockTag)
	if err != nil {
		log.Fatalf("failed to resolve block: %v", err)
	}

	proxyCode, err := fetchCode(ctx, client, address, blockTag)
	if err != nil {
		log.Fatalf("failed to fetch proxy code: %v", err)
	}

	implAddr, err := fetchImplementationAddress(ctx, client, address, blockTag)
	if err != nil {
		log.Fatalf("failed to fetch implementation address: %v", err)
	}

	proxyStorage, err := fetchStorage(ctx, client, blockHash, txIndex, common.HexToAddress(address), maxResults)
	if err != nil {
		log.Fatalf("failed to fetch proxy storage: %v", err)
	}

	var (
		verifierMappings  []verifierMapping
		verifierAddresses []common.Address
	)
	if len(configDigests) > 0 {
		verifierMappings, verifierAddresses, err = loadVerifierMappingsFromDigests(ctx, client, common.HexToAddress(address), configDigests, blockTag)
		if err != nil {
			log.Fatalf("failed to fetch verifier mappings from digests: %v", err)
		}
	} else {
		verifierMappings, verifierAddresses, err = loadVerifierMappingsFromStorage(ctx, client, proxyStorage, blockTag)
		if err != nil {
			log.Fatalf("failed to fetch verifier mappings from storage: %v", err)
		}
	}
	if err != nil {
		log.Fatalf("failed to fetch verifier mappings: %v", err)
	}

	verifierDumps, err := fetchVerifierDumps(ctx, client, blockHash, txIndex, blockTag, verifierAddresses, maxResults)
	if err != nil {
		log.Fatalf("failed to fetch verifier dumps: %v", err)
	}

	var implementationDump *contractDump
	if implAddr != (common.Address{}) {
		implCode, err := fetchCode(ctx, client, implAddr.Hex(), blockTag)
		if err != nil {
			log.Fatalf("failed to fetch implementation code: %v", err)
		}
		implStorage, err := fetchStorage(ctx, client, blockHash, txIndex, implAddr, maxResults)
		if err != nil {
			log.Fatalf("failed to fetch implementation storage: %v", err)
		}
		implementationDump = &contractDump{
			Address: implAddr.Hex(),
			Code:    implCode,
			Storage: implStorage,
		}
	}

	dump := verifierDump{
		Proxy: contractDump{
			Address: common.HexToAddress(address).Hex(),
			Code:    proxyCode,
			Storage: proxyStorage,
		},
		Implementation:   implementationDump,
		VerifierMappings: verifierMappings,
		Verifiers:        verifierDumps,
		Meta: dumpMeta{
			RPCAddress:            rpcAddr,
			BlockTag:              blockTag,
			BlockHash:             blockHash.Hex(),
			TxIndex:               txIndex,
			ImplementationSlot:    eip1967ImplementationSlot,
			ImplementationFound:   implAddr != (common.Address{}),
			VerifierMappingSource: verifierMappingSource(configDigests),
		},
	}

	out, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}

	if outPath == "" {
		fmt.Printf("%s\n", out)
		return
	}

	if err := os.WriteFile(outPath, out, 0o600); err != nil {
		log.Fatalf("failed to write output: %v", err)
	}
}

func resolveBlock(ctx context.Context, client *rpc.Client, blockTag string) (common.Hash, uint64, error) {
	var header blockHeader
	if err := client.CallContext(ctx, &header, "eth_getBlockByNumber", blockTag, false); err != nil {
		return common.Hash{}, 0, err
	}
	if header.Hash == (common.Hash{}) {
		return common.Hash{}, 0, fmt.Errorf("block hash not found for tag %q", blockTag)
	}

	var txCount hexutil.Uint
	if err := client.CallContext(ctx, &txCount, "eth_getBlockTransactionCountByHash", header.Hash); err != nil {
		return common.Hash{}, 0, err
	}

	return header.Hash, uint64(txCount), nil
}

func fetchCode(ctx context.Context, client *rpc.Client, address, blockTag string) (string, error) {
	var code hexutil.Bytes
	if err := client.CallContext(ctx, &code, "eth_getCode", address, blockTag); err != nil {
		return "", err
	}
	return hexutil.Encode(code), nil
}

func fetchImplementationAddress(ctx context.Context, client *rpc.Client, address, blockTag string) (common.Address, error) {
	slot := common.HexToHash(eip1967ImplementationSlot)
	var raw hexutil.Bytes
	if err := client.CallContext(ctx, &raw, "eth_getStorageAt", address, slot, blockTag); err != nil {
		return common.Address{}, err
	}

	padded := make([]byte, 32)
	copy(padded[32-len(raw):], raw)
	implAddr := common.BytesToAddress(padded[12:])
	if implAddr == (common.Address{}) {
		return common.Address{}, nil
	}
	return implAddr, nil
}

func fetchStorageValue(ctx context.Context, client *rpc.Client, address common.Address, slot common.Hash, blockTag string) (string, error) {
	var raw hexutil.Bytes
	if err := client.CallContext(ctx, &raw, "eth_getStorageAt", address, slot, blockTag); err != nil {
		return "", err
	}
	return hexutil.Encode(raw), nil
}

func fetchStorage(ctx context.Context, client *rpc.Client, blockHash common.Hash, txIndex uint64, address common.Address, maxResults uint64) ([]storageEntry, error) {
	storage := make(map[string]string)
	startKey := normalizeStorageKey("0x00")

	for {
		var result storageRangeResult
		if err := client.CallContext(ctx, &result, "debug_storageRangeAt", blockHash, txIndex, address, startKey, maxResults); err != nil {
			return nil, err
		}

		for key, entry := range result.Storage {
			storageKey := entry.Key
			if storageKey == "" {
				storageKey = key
			}
			storage[normalizeHex(storageKey)] = normalizeHex(entry.Value)
		}

		if result.NextKey == "" {
			break
		}
		startKey = normalizeStorageKey(result.NextKey)
	}

	keys := make([]string, 0, len(storage))
	for key := range storage {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]storageEntry, 0, len(keys))
	for _, key := range keys {
		out = append(out, storageEntry{
			Key:   key,
			Value: storage[key],
		})
	}

	return out, nil
}

func normalizeHex(value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "0x") {
		return value
	}
	return "0x" + value
}

func verifierMappingSource(digests configDigestList) string {
	if len(digests) > 0 {
		return "config-digest"
	}
	return "storage"
}

func normalizeStorageKey(value string) string {
	if value == "" {
		return value
	}
	if !strings.HasPrefix(value, "0x") {
		value = "0x" + value
	}
	hexPart := value[2:]
	if len(hexPart)%2 == 1 {
		value = "0x0" + hexPart
	}
	return value
}

func parseConfigDigest(value string) (common.Hash, error) {
	if value == "" {
		return common.Hash{}, fmt.Errorf("config digest is empty")
	}
	if !strings.HasPrefix(value, "0x") {
		value = "0x" + value
	}
	raw, err := hexutil.Decode(value)
	if err != nil {
		return common.Hash{}, fmt.Errorf("invalid config digest %q: %w", value, err)
	}
	if len(raw) != 32 {
		return common.Hash{}, fmt.Errorf("invalid config digest length %d for %q", len(raw), value)
	}
	return common.BytesToHash(raw), nil
}

func loadVerifierMappingsFromStorage(ctx context.Context, client *rpc.Client, proxyStorage []storageEntry, blockTag string) ([]verifierMapping, []common.Address, error) {
	verifierCallData := buildSupportsInterfaceCallData(verifierInterfaceID())
	cache := make(map[common.Address]bool)
	addresses := make(map[common.Address]struct{})
	mappings := make([]verifierMapping, 0)

	for _, entry := range proxyStorage {
		addr, ok := addressFromStorageValue(entry.Value)
		if !ok {
			continue
		}
		isVerifier, err := isVerifierContract(ctx, client, addr, blockTag, verifierCallData, cache)
		if err != nil {
			return nil, nil, err
		}
		if !isVerifier {
			continue
		}
		mappings = append(mappings, verifierMapping{
			StorageKey: entry.Key,
			Verifier:   addr.Hex(),
		})
		addresses[addr] = struct{}{}
	}

	sort.Slice(mappings, func(i, j int) bool {
		if mappings[i].StorageKey != mappings[j].StorageKey {
			return mappings[i].StorageKey < mappings[j].StorageKey
		}
		return mappings[i].Verifier < mappings[j].Verifier
	})

	outAddresses := make([]common.Address, 0, len(addresses))
	for verifier := range addresses {
		outAddresses = append(outAddresses, verifier)
	}
	sort.Slice(outAddresses, func(i, j int) bool {
		return bytes.Compare(outAddresses[i].Bytes(), outAddresses[j].Bytes()) < 0
	})

	return mappings, outAddresses, nil
}

func loadVerifierMappingsFromDigests(ctx context.Context, client *rpc.Client, proxyAddress common.Address, digests []common.Hash, blockTag string) ([]verifierMapping, []common.Address, error) {
	addresses := make(map[common.Address]struct{})
	mappings := make([]verifierMapping, 0, len(digests))

	for _, digest := range digests {
		storageKey := computeMappingStorageKey(digest, verifierMappingSlot)
		value, err := fetchStorageValue(ctx, client, proxyAddress, storageKey, blockTag)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch mapping value for %s: %w", digest.Hex(), err)
		}
		addr, _ := addressFromStorageValue(value)
		mappings = append(mappings, verifierMapping{
			ConfigDigest: digest.Hex(),
			StorageKey:   storageKey.Hex(),
			Verifier:     addr.Hex(),
		})
		if addr != (common.Address{}) {
			addresses[addr] = struct{}{}
		}
	}

	sort.Slice(mappings, func(i, j int) bool {
		if mappings[i].ConfigDigest != mappings[j].ConfigDigest {
			return mappings[i].ConfigDigest < mappings[j].ConfigDigest
		}
		return mappings[i].StorageKey < mappings[j].StorageKey
	})

	outAddresses := make([]common.Address, 0, len(addresses))
	for verifier := range addresses {
		outAddresses = append(outAddresses, verifier)
	}
	sort.Slice(outAddresses, func(i, j int) bool {
		return bytes.Compare(outAddresses[i].Bytes(), outAddresses[j].Bytes()) < 0
	})

	return mappings, outAddresses, nil
}

func fetchVerifierDumps(ctx context.Context, client *rpc.Client, blockHash common.Hash, txIndex uint64, blockTag string, verifierAddresses []common.Address, maxResults uint64) ([]contractDump, error) {
	dumps := make([]contractDump, 0, len(verifierAddresses))
	for _, verifierAddress := range verifierAddresses {
		code, err := fetchCode(ctx, client, verifierAddress.Hex(), blockTag)
		if err != nil {
			return nil, fmt.Errorf("fetch verifier code %s: %w", verifierAddress.Hex(), err)
		}
		storage, err := fetchStorage(ctx, client, blockHash, txIndex, verifierAddress, maxResults)
		if err != nil {
			return nil, fmt.Errorf("fetch verifier storage %s: %w", verifierAddress.Hex(), err)
		}
		dumps = append(dumps, contractDump{
			Address: verifierAddress.Hex(),
			Code:    code,
			Storage: storage,
		})
	}
	return dumps, nil
}

func isVerifierContract(ctx context.Context, client *rpc.Client, address common.Address, blockTag string, callData []byte, cache map[common.Address]bool) (bool, error) {
	if cached, ok := cache[address]; ok {
		return cached, nil
	}

	code, err := fetchCode(ctx, client, address.Hex(), blockTag)
	if err != nil {
		return false, err
	}
	if code == "0x" {
		cache[address] = false
		return false, nil
	}

	ok, err := supportsInterface(ctx, client, address, blockTag, callData)
	if err != nil {
		cache[address] = false
		return false, nil
	}
	cache[address] = ok
	return ok, nil
}

func supportsInterface(ctx context.Context, client *rpc.Client, address common.Address, blockTag string, callData []byte) (bool, error) {
	var result hexutil.Bytes
	call := map[string]interface{}{
		"to":   address.Hex(),
		"data": hexutil.Encode(callData),
	}
	if err := client.CallContext(ctx, &result, "eth_call", call, blockTag); err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, nil
	}
	padded := make([]byte, 32)
	copy(padded[32-len(result):], result)
	return padded[31] == 1, nil
}

func verifierInterfaceID() [4]byte {
	hash := crypto.Keccak256([]byte(verifierInterfaceSignature))
	var id [4]byte
	copy(id[:], hash[:4])
	return id
}

func buildSupportsInterfaceCallData(interfaceID [4]byte) []byte {
	selector := crypto.Keccak256([]byte("supportsInterface(bytes4)"))
	data := make([]byte, 4+32)
	copy(data[:4], selector[:4])
	copy(data[4+28:], interfaceID[:])
	return data
}

func computeMappingStorageKey(key common.Hash, slot uint64) common.Hash {
	slotBytes := common.LeftPadBytes(new(big.Int).SetUint64(slot).Bytes(), 32)
	data := make([]byte, 64)
	copy(data[:32], key.Bytes())
	copy(data[32:], slotBytes)
	return crypto.Keccak256Hash(data)
}

func addressFromStorageValue(value string) (common.Address, bool) {
	if value == "" {
		return common.Address{}, false
	}
	raw, err := hexutil.Decode(value)
	if err != nil {
		return common.Address{}, false
	}
	if len(raw) > 32 {
		return common.Address{}, false
	}
	padded := make([]byte, 32)
	copy(padded[32-len(raw):], raw)
	for _, b := range padded[:12] {
		if b != 0 {
			return common.Address{}, false
		}
	}
	addr := common.BytesToAddress(padded[12:])
	if addr == (common.Address{}) {
		return common.Address{}, false
	}
	return addr, true
}
