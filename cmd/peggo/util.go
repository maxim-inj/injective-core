package main

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
)

// readEnv is a special utility that reads `.env` file into actual environment variables of the current app
func readEnv() {
	if err := godotenv.Load(); err != nil {
		log.WithError(err).Warningln("failed to load .env file")
	}
}

// stdinConfirm checks the user's confirmation, if not forced to Yes
func stdinConfirm(msg string) bool {
	var response string

	fmt.Print(msg)

	if _, err := fmt.Scanln(&response); err != nil {
		log.WithError(err).Errorln("failed to confirm the action")
		return false
	}

	switch strings.ToLower(strings.TrimSpace(response)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// parseERC20ContractMapping converts list of address:denom pairs to a proper typed map.
func parseERC20ContractMapping(items []string) map[ethcmn.Address]string {
	res := make(map[ethcmn.Address]string)

	for _, item := range items {
		// item is a pair address:denom
		parts := strings.Split(item, ":")
		addr := ethcmn.HexToAddress(parts[0])

		if len(parts) != 2 || len(parts[0]) == 0 || addr == (ethcmn.Address{}) {
			log.Fatalln("failed to parse ERC20 mapping: check that all inputs contain valid denom:address pairs")
		}

		denom := parts[1]
		res[addr] = denom
	}

	return res
}

// logLevel converts vague log level name into typed level.
func logLevel(s string) log.Level {
	switch s {
	case "1", "error":
		return log.ErrorLevel
	case "2", "warn":
		return log.WarnLevel
	case "3", "info":
		return log.InfoLevel
	case "4", "debug":
		return log.DebugLevel
	default:
		return log.FatalLevel
	}
}

// toBool is used to parse vague bool definition into typed bool.
func toBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "t", "yes":
		return true
	default:
		return false
	}
}

// duration parses duration from string with a provided default fallback.
func duration(s string, defaults time.Duration) time.Duration {
	dur, err := time.ParseDuration(s)
	if err != nil {
		dur = defaults
	}
	return dur
}

// checkStatsdPrefix ensures that the statsd prefix really
// have "." at end.
func checkStatsdPrefix(s string) string {
	if !strings.HasSuffix(s, ".") {
		return s + "."
	}
	return s
}

func hexToBytes(str string) ([]byte, error) {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}

	data, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// orShutdown fatals the app if there was an error.
func orShutdown(err error) {
	if err != nil && err != grpc.ErrServerStopped {
		log.WithError(err).Fatalln("unable to start peggo")
	}
}
