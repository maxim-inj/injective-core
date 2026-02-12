package helpers

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/InjectiveLabs/sdk-go/chain/crypto/ethsecp256k1"
)

// GenerateEthereumPrivateKey creates an Ethereum private key from a mnemonic
// using Injective's eth_secp256k1 derivation algorithm
func GenerateEthereumPrivateKey(mnemonic string) (*ecdsa.PrivateKey, error) {
	privKey, err := NewPrivKeyFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}

	// Cast to ethsecp256k1 and convert to ECDSA
	ethPrivKey, ok := privKey.(*ethsecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("private key should be ethsecp256k1 type")
	}

	return ethPrivKey.ToECDSA(), nil
}
