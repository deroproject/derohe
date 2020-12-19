// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package address

import "fmt"

//import "bytes"
//import "encoding/binary"

//import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/crypto"

// older dero address https://cryptonote.org/cns/cns007.txt to understand address more
// current dero versions use https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki

// there are 3 hrps for mainnet DERO, DEROI,DEROPROOF
// these are 3 hrps for testnet DETO, DETOI,DEROPROOF
// so a total of 5 hrps
// 1 byte version is support capable to represent 255 versions, currently we will be using only version 1
// point is 33 bytes
// payment id if present is 8 bytes

type Address struct {
	Network   uint64
	Amount    uint64 // integrated address can also contain amount to send
	Mainnet   bool
	Proof     bool
	PublicKey *crypto.Point //33 byte compressed point
	PaymentID []byte        //integrated payment id is 8 bytes, It is encrypted on the blockchain
}

// SegwitAddrEncode encodes hrp(human-readable part) , version(int) and data(bytes array), returns Segwit Address / or error
func (a *Address) DeroAddrEncode() (string, error) {
	hrp := "dero"
	if !a.Mainnet {
		hrp = "deto"
	}

	if len(a.PaymentID) == 8 {
		hrp += "i"
	}

	if a.Proof {
		hrp = "deroproof"
	}

	// first we are appending version byte
	data_bytes := append([]byte{1}, a.PublicKey.EncodeCompressed()...)
	data_bytes = append(data_bytes, a.PaymentID...)
	var data_ints []int
	for i := range data_bytes {
		data_ints = append(data_ints, int(data_bytes[i]))
	}

	data, err := convertbits(data_ints, 8, 5, true)
	if err != nil {
		return "", err
	}
	ret, err := Encode(hrp, data)
	if err != nil {
		return "", err
	}
	return ret, nil
}

// stringifier
func (a Address) String() string {
	result, _ := a.DeroAddrEncode()
	return result
}

// split address into parts
func (a Address) Split() (z Address, payment_id []byte) {
	z = a
	z.PaymentID = []byte{}
	if len(z.PaymentID) == 8 {
		payment_id = append(payment_id, a.PaymentID...)
	}
	return z, payment_id
}

// if address has amount
func (a Address) IntegratedAmount() uint64 {
	return a.Amount
}

func (a Address) Compressed() []byte {
	return a.PublicKey.EncodeCompressed()
}

// tells whether address is mainnet address
func (a *Address) IsMainnet() bool {
	return a.Mainnet
}

// tells whether address is mainnet address
func (a *Address) IsIntegratedAddress() bool {
	return len(a.PaymentID) > 0
}

// tells whether address belongs to DERO Network
func (a *Address) IsDERONetwork() bool {
	return true
}

// NewAddress decodes hrp(human-readable part) Address(string), returns address  or error
func NewAddress(addr string) (result *Address, err error) {
	dechrp, data, err := Decode(addr)
	if err != nil {
		return nil, err
	}
	result = &Address{PublicKey: new(crypto.Point)}
	switch dechrp {
	case "dero", "deroi", "deto", "detoi", "deroproof":
	default:
		return nil, fmt.Errorf("invalid human-readable part : %s != %s", "", dechrp)
	}
	if len(data) < 1 {
		return nil, fmt.Errorf("invalid decode version: %d", len(data))
	}

	res, err := convertbits(data, 5, 8, false)
	if err != nil {
		return nil, err
	}

	if res[0] != 1 {
		return nil, fmt.Errorf("invalid address version : %d", res[0])
	}
	res = res[1:]

	var resbytes []byte
	for _, b := range res {
		resbytes = append(resbytes, byte(b))
	}

	if len(resbytes) < 33 {
		return nil, fmt.Errorf("invalid address length as per spec : %d", len(res))
	}

	if err = result.PublicKey.DecodeCompressed(resbytes[0:33]); err != nil {
		return
	}

	result.Mainnet = true
	if dechrp == "deto" || dechrp == "detoi" {
		result.Mainnet = false
	}
	if dechrp == "deroproof" {
		result.Proof = true
	}

	switch {
	case len(res) == 33 && (dechrp == "dero" || dechrp == "deto"):
	case len(res) == 33+8 && (dechrp == "deroi" || dechrp == "detoi" || dechrp == "deroproof"): // address only has payment id
		result.PaymentID = append(result.PaymentID, resbytes[33:33+8]...)

	case len(res) == 33+8+8 && (dechrp == "deroi" || dechrp == "detoi"): // address has payment id and amount support
	default:
		return nil, fmt.Errorf("invalid address length as per spec : %d", len(res))
	}

	return result, nil
}

// create a new address from key
func NewAddressFromKeys(key *crypto.Point) (result *Address) {
	result = &Address{
		Mainnet:   true,
		PublicKey: new(crypto.Point).Set(key),
	}
	return
}
