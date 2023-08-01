package blockchain_crypto

import (
	"fmt"
	"math/big"
)

type Signature struct {
	R *big.Int
	S *big.Int
}

func (s *Signature) String() string {
	return fmt.Sprintf("%064x%064x", s.R, s.S)
}

func SignatureStrToSignature(signatureStr string) *Signature {
	r, s := StringToBigIntTuple(signatureStr)
	return &Signature{
		R: r,
		S: s,
	}
}
