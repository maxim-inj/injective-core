package utils

import (
	"strings"

	"cosmossdk.io/math"
)

// getReadableDec is a test utility function to return a readable representation of decimal strings
func GetReadableDec(d math.LegacyDec) string {
	if d.IsNil() {
		return d.String()
	}
	dec := strings.TrimRight(d.String(), "0")
	if len(dec) < 2 {
		return dec
	}

	if dec[len(dec)-1:] == "." {
		return dec + "0"
	}
	return dec
}
