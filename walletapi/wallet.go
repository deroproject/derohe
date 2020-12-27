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

package walletapi

import "fmt"
import "sort"
import "sync"
import "time"
import "bytes"
import "strings"
import "math/big"
import "crypto/rand"

//import "encoding/json"
//import "encoding/binary"

//import "github.com/romana/rlog"
//import "github.com/vmihailenco/msgpack"

//import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/structures"
import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/crypto/bn256"

//import "github.com/deroproject/derosuite/crypto/ringct"
//import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/walletapi/mnemonics"
import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derohe/blockchain/inputmaturity"

type _Keys struct {
	Secret *crypto.BNRed `json:"secret"`
	Public *crypto.Point `json:"public"`
}

var Balance_lookup_table *LookupTable

type Account struct {
	Keys           _Keys   `json:"keys"`
	SeedLanguage   string  `json:"seedlanguage"`
	FeesMultiplier float32 `json:"feesmultiplier"` // fees multiplier accurate to 2 decimals
	Ringsize       int     `json:"ringsize"`       // default mixn to use for txs
	mainnet        bool

	Height     uint64 `json:"height"`     // block height till where blockchain has been scanned
	TopoHeight int64  `json:"topoheight"` // block height till where blockchain has been scanned

	Balance_Mature uint64 `json:"balance_mature"` // total balance of account
	Balance_Locked uint64 `json:"balance_locked"` // balance locked

	Balance_Result structures.GetEncryptedBalance_Result // used to cache last successful result

	Entries []Entry // all tx entries, basically transaction statement

	RingMembers map[string]int64 `json:"ring_members"` // ring members

	Pool Wallet_Pool // wallet pool

	sync.Mutex // syncronise modifications to this structure
}

// these structures are completely decoupled from blockchain and live only within the wallet
// all inputs and outputs which modify balance are presented by this structure
type Entry struct {
	Height         uint64                               `json:"height"`
	TopoHeight     int64                                `json:"topoheight"`
	BlockHash      string                               `json:"blockhash"`
	MinerReward    uint64                               `json:"minerreward"`
	TransactionPos int                                  `json:"poswithinblock"` // pos within block is negative for coinbase
	Coinbase       bool                                 `json:"coinbase"`
	Incoming       bool                                 `json:"incoming"`
	TXID           crypto.Hash                          `json:"txid"`
	Amount         uint64                               `json:"amount"`
	Fees           uint64                               `json:"fees"`
	PaymentID      []byte                               `json:"payment_id"`
	Proof          string                               `json:"proof"`
	Status         byte                                 `json:"status"`
	Unlock_Time    uint64                               `json:"unlock_time"`
	Time           time.Time                            `json:"time"`
	EWData         string                               `json:"ewdata"`        // encrypted wallet balance at that point in time
	Secret_TX_Key  string                               `json:"secret_tx_key"` // can be used to prove if available
	Details        structures.Outgoing_Transfer_Details `json:"details"`       // actual details if available
}

// add a entry in the suitable place
// this is always single threaded
func (w *Wallet_Memory) InsertReplace(e Entry) {

	i := sort.Search(len(w.account.Entries), func(j int) bool {
		return w.account.Entries[j].TopoHeight >= e.TopoHeight && w.account.Entries[j].TransactionPos >= e.TransactionPos
	})

	// entry already exists, we are probably rescanning/overwiting, delete anything afterwards
	if i < len(w.account.Entries) && w.account.Entries[i].TopoHeight == e.TopoHeight && w.account.Entries[i].TransactionPos == e.TransactionPos {
		w.account.Entries = w.account.Entries[:i]
		// x is present at data[i]
	} else {
		// x is not present in data,
		// but i is the index where it would be inserted.
	}
	w.account.Entries = append(w.account.Entries, e)

}

// generate keys from using random numbers
func Generate_Keys_From_Random() (user *Account, err error) {
	user = &Account{Ringsize: 4, FeesMultiplier: 1.5}
	seed := crypto.RandomScalarBNRed()
	user.Keys = Generate_Keys_From_Seed(seed)

	return
}

// generate keys from seed which is from the recovery words
// or we feed in direct
func Generate_Keys_From_Seed(Seed *crypto.BNRed) (keys _Keys) {

	// setup main keys
	keys.Secret = Seed
	keys.Public = crypto.GPoint.ScalarMult(Seed)

	return
}

// generate user account using recovery seeds
func Generate_Account_From_Recovery_Words(words string) (user *Account, err error) {
	user = &Account{Ringsize: 4, FeesMultiplier: 1.5}
	language, seed, err := mnemonics.Words_To_Key(words)
	if err != nil {
		return
	}

	user.SeedLanguage = language
	user.Keys = Generate_Keys_From_Seed(crypto.GetBNRed(seed))

	return
}

func Generate_Account_From_Seed(Seed *crypto.BNRed) (user *Account, err error) {
	user = &Account{Ringsize: 4, FeesMultiplier: 1.5}

	// TODO check whether the seed is invalid
	user.Keys = Generate_Keys_From_Seed(Seed)

	return
}

// convert key to seed using language
func (w *Wallet_Memory) GetSeed() (str string) {
	return mnemonics.Key_To_Words(w.account.Keys.Secret.BigInt(), w.account.SeedLanguage)
}

// convert key to seed using language
func (w *Wallet_Memory) GetSeedinLanguage(lang string) (str string) {
	return mnemonics.Key_To_Words(w.account.Keys.Secret.BigInt(), lang)
}

func (account *Account) GetAddress() (addr address.Address) {
	addr.PublicKey = account.Keys.Public
	return
}

// convert a user account to address
func (w *Wallet_Memory) GetAddress() (addr address.Address) {
	addr = w.account.GetAddress()
	addr.Mainnet = w.account.mainnet
	return addr
}

// get a random integrated address
func (w *Wallet_Memory) GetRandomIAddress8() (addr address.Address) {
	addr = w.GetAddress()

	// setup random 8 bytes of payment ID, it must be from non-deterministic RNG namely crypto random
	addr.PaymentID = make([]byte, 8, 8)
	rand.Read(addr.PaymentID[:])

	return
}

func (w *Wallet_Memory) Get_Balance_Rescan() (mature_balance uint64, locked_balance uint64) {
	return w.Get_Balance()
}

// get the unlocked balance ( amounts which are mature and can be spent at this time )
// offline wallets may get this wrong, since they may not have latest data

//
func (w *Wallet_Memory) Get_Balance() (mature_balance uint64, locked_balance uint64) {
	return w.account.Balance_Mature, 0

}

// finds all inputs which have been received/spent etc
// TODO this code can be easily parallelised and need to be parallelised
// if only the availble is requested, then the wallet is very fast
// the spent tracking may make it slow ( in case of large probably million  txs )
//TODO currently we do not track POOL at all any where ( except while building tx)
// if payment_id is true, only entries with payment ids are returned
// min_height/max height represent topoheight
func (w *Wallet_Memory) Show_Transfers(available bool, in bool, out bool, pool bool, failed bool, payment_id bool, min_height, max_height uint64) (entries []Entry) {

	// dero_first_block_time := time.Unix(1512432000, 0) //Tuesday, December 5, 2017 12:00:00 AM

	if max_height == 0 {
		max_height = 50000000000
	}

	for _, e := range w.account.Entries {
		if e.Height >= min_height && e.Height <= max_height {
			if in && (e.Incoming || e.Coinbase) {

				if payment_id && len(e.PaymentID) >= 8 {
					entries = append(entries, e)
				} else {
					entries = append(entries, e)
				}
				continue
			}
			if out && !(e.Incoming || e.Coinbase) {
				if payment_id && len(e.PaymentID) >= 8 {
					entries = append(entries, e)
				} else {
					entries = append(entries, e)
				}
				continue
			}
		}
	}

	return

}

// gets all the payments  done to specific payment ID and filtered by specific block height
// we do need better structures
func (w *Wallet_Memory) Get_Payments_Payment_ID(payid []byte, min_height uint64) (entries []Entry) {
	for _, e := range w.account.Entries {
		if e.Height >= min_height {
			if bytes.Compare(payid, e.PaymentID[:]) == 0 {
				entries = append(entries, e)
			}
		}
	}

	return

}

// return all payments within a tx there can be only 1 entry
// NOTE: what about multiple payments
func (w *Wallet_Memory) Get_Payments_TXID(txid []byte) (entry Entry) {
	for _, e := range w.account.Entries {
		if bytes.Compare(txid, e.TXID[:]) == 0 {
			return e
		}
	}

	return
}

// delete most of the data and prepare for rescan
func (w *Wallet_Memory) Clean() {
	w.account.Entries = w.account.Entries[:0]
	w.account.Balance_Result.Data = ""
}

// return height of wallet
func (w *Wallet_Memory) Get_Height() uint64 {
	return uint64(w.account.Balance_Result.Height)
}

// return topoheight of wallet
func (w *Wallet_Memory) Get_TopoHeight() int64 {
	return w.account.Balance_Result.Topoheight
}

func (w *Wallet_Memory) Get_Daemon_Height() uint64 {
	return w.Daemon_Height
}

// return topoheight of darmon
func (w *Wallet_Memory) Get_Daemon_TopoHeight() int64 {
	return w.Daemon_TopoHeight
}

func (w *Wallet_Memory) Get_Registration_TopoHeight() int64 {
	return w.account.Balance_Result.Registration
}

func (w *Wallet_Memory) Get_Keys() _Keys {
	return w.account.Keys
}

// by default a wallet opens in Offline Mode
// however, if the wallet is in online mode, it can be made offline instantly using this
func (w *Wallet_Memory) SetOfflineMode() bool {
	current_mode := w.wallet_online_mode
	w.wallet_online_mode = false
	return current_mode
}

func (w *Wallet_Memory) SetNetwork(mainnet bool) bool {
	w.account.mainnet = mainnet
	return w.account.mainnet
}

func (w *Wallet_Memory) GetNetwork() bool {
	return w.account.mainnet
}

// return current mode
func (w *Wallet_Memory) GetMode() bool {
	return w.wallet_online_mode
}

// use the endpoint set  by the program
func (w *Wallet_Memory) SetDaemonAddress(endpoint string) string {
	w.Daemon_Endpoint = endpoint
	return w.Daemon_Endpoint
}

// by default a wallet opens in Offline Mode
// however, It can be made online by calling this
func (w *Wallet_Memory) SetOnlineMode() bool {
	current_mode := w.wallet_online_mode
	w.wallet_online_mode = true

	if current_mode != true { // trigger subroutine if previous mode was offline
		go w.sync_loop() // start sync subroutine
	}
	return current_mode
}

// by default a wallet opens in Offline Mode
// however, It can be made online by calling this
func (w *Wallet_Memory) SetRingSize(ringsize int) int {
	defer w.Save_Wallet() // save wallet

	if ringsize >= 2 && ringsize <= 128 { //reasonable limits for mixin, atleastt for now, network should bump it to 13 on next HF

		if crypto.IsPowerOf2(ringsize) {
			w.account.Ringsize = ringsize
		}
	}
	return w.account.Ringsize
}

// by default a wallet opens in Offline Mode
// however, It can be made online by calling this
func (w *Wallet_Memory) GetRingSize() int {
	if w.account.Ringsize < 2 {
		return 2
	}
	return w.account.Ringsize
}

// sets a fee multiplier
func (w *Wallet_Memory) SetFeeMultiplier(x float32) float32 {
	defer w.Save_Wallet() // save wallet
	if x < 1.0 {          // fee cannot be less than 1.0, base fees
		w.account.FeesMultiplier = 2.0
	} else {
		w.account.FeesMultiplier = x
	}
	return w.account.FeesMultiplier
}

// gets current fee multiplier
func (w *Wallet_Memory) GetFeeMultiplier() float32 {
	if w.account.FeesMultiplier < 1.0 {
		return 1.0
	}
	return w.account.FeesMultiplier
}

// get fees multiplied by multiplier
func (w *Wallet_Memory) getfees(txfee uint64) uint64 {
	multiplier := w.account.FeesMultiplier
	if multiplier < 1.0 {
		multiplier = 2.0
	}
	return txfee * uint64(multiplier*100.0) / 100
}

// Ability to change seed lanaguage
func (w *Wallet_Memory) SetSeedLanguage(language string) string {
	defer w.Save_Wallet() // save wallet

	language_list := mnemonics.Language_List()
	for i := range language_list {
		if strings.ToLower(language) == strings.ToLower(language_list[i]) {
			w.account.SeedLanguage = language_list[i]
		}
	}
	return w.account.SeedLanguage
}

// retrieve current seed language
func (w *Wallet_Memory) GetSeedLanguage() string {
	if w.account.SeedLanguage == "" { // default is English
		return "English"
	}
	return w.account.SeedLanguage
}

// retrieve  secret key for any tx we may have created
func (w *Wallet_Memory) GetRegistrationTX() *transaction.Transaction {
	var tx transaction.Transaction
	tx.Version = 1
	tx.TransactionType = transaction.REGISTRATION
	add := w.account.Keys.Public.EncodeCompressed()
	copy(tx.MinerAddress[:], add[:])
	c, s := w.sign()
	crypto.FillBytes(c, tx.C[:])
	crypto.FillBytes(s, tx.S[:])

	if !tx.IsRegistrationValid() {
		panic("registration tx could not be generated. something failed.")
	}

	return &tx
}

// this basically does a  Schnorr Signature on random information for registration
func (w *Wallet_Memory) sign() (c, s *big.Int) {
	var tmppoint bn256.G1

	tmpsecret := crypto.RandomScalar()
	tmppoint.ScalarMult(crypto.G, tmpsecret)

	serialize := []byte(fmt.Sprintf("%s%s", w.account.Keys.Public.G1().String(), tmppoint.String()))
	c = crypto.ReducedHash(serialize)
	s = new(big.Int).Mul(c, w.account.Keys.Secret.BigInt()) // basicaly scalar mul add
	s = s.Mod(s, bn256.Order)
	s = s.Add(s, tmpsecret)
	s = s.Mod(s, bn256.Order)

	return
}

// retrieve  secret key for any tx we may have created
func (w *Wallet_Memory) GetTXKey(txhash crypto.Hash) string {
	for _, e := range w.account.Entries {
		if !e.Coinbase && !e.Incoming && e.TXID == txhash {
			return e.Proof
		}
	}

	return ""
}
