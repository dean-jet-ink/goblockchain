package blockchain_crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
)

func StringToBigIntTuple(str string) (*big.Int, *big.Int) {
	bX, _ := hex.DecodeString(str[:64])
	bY, _ := hex.DecodeString(str[64:])

	var x big.Int
	var y big.Int

	x.SetBytes(bX)
	y.SetBytes(bY)

	return &x, &y
}

func PublicKeyStrToPublicKey(publicKeyStr string) *ecdsa.PublicKey {
	x, y := StringToBigIntTuple(publicKeyStr)
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
}

func PrivateKeyStrToPrivateKey(privateKeyStr string, publicKey *ecdsa.PublicKey) *ecdsa.PrivateKey {
	bD, _ := hex.DecodeString(privateKeyStr)
	var d big.Int
	d.SetBytes(bD)

	return &ecdsa.PrivateKey{
		PublicKey: *publicKey,
		D:         &d,
	}
}
