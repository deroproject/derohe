package pow

//import "crypto/sha256"
import (
	"github.com/stratumfarm/derohe/astrobwt"
	"github.com/stratumfarm/derohe/cryptography/crypto"
)

// patch algorithm in here to conduct various tests
func Pow(input []byte) (output crypto.Hash) {
	//return SimplePow(input)
	return astrobwt.POW16(input)
}

// replace with a different pow
/*
func SimplePow(input []byte) (output crypto.Hash) {
	return crypto.Hash(sha256.Sum256(input))
}
*/
