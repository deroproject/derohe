package pow

//import "crypto/sha256"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/astrobwt"

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
