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

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
	"github.com/deroproject/derohe/walletapi/mnemonics"
	"github.com/go-logr/logr"
)

//import "github.com/deroproject/derohe/blockchain/inputmaturity"

var logger logr.Logger = logr.Discard() // default discard all logs

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
	Registered     bool `json:"registered"`

	Height     uint64 `json:"height"`     // block height till where blockchain has been scanned
	TopoHeight int64  `json:"topoheight"` // block height till where blockchain has been scanned

	Balance_Mature uint64                 `json:"balance_mature"` // total balance of account
	Balance        map[crypto.Hash]uint64 `json:"balance"`        // balance of account and other private scs
	Balance_Locked uint64                 `json:"balance_locked"` // balance locked

	Balance_Result []rpc.GetEncryptedBalance_Result // used to cache last successful result

	//Entries []rpc.Entry // all tx entries, basically transaction statement

	EntriesNative map[crypto.Hash][]rpc.Entry // all subtokens are stored here

	RingMembers map[string]int64 `json:"ring_members"` // ring members

	SaveChangesEvery time.Duration `json:"-"` // default is zero
	lastsaved        time.Time

	// do not build entire history from 0, only maintain top history
	TrackRecentBlocks int64 `json:"-"` // only scan top blocks, default is zero, means everything

	sync.Mutex // syncronise modifications to this structure
}

func (w *Wallet_Memory) getEncryptedBalanceresult(scid crypto.Hash) rpc.GetEncryptedBalance_Result {
	for _, e := range w.account.Balance_Result {
		if scid == e.SCID {
			return e
		}
	}
	return rpc.GetEncryptedBalance_Result{}
}

func (w *Wallet_Memory) setEncryptedBalanceresult(scid crypto.Hash, entry rpc.GetEncryptedBalance_Result) {
	for i, e := range w.account.Balance_Result {
		if scid == e.SCID {
			w.account.Balance_Result[i] = entry
			return
		}
	}
	w.account.Balance_Result = append(w.account.Balance_Result, entry)
}

// add a entry in the suitable place
// this is always single threaded
func (w *Wallet_Memory) InsertReplace(scid crypto.Hash, e rpc.Entry) {
	var entries []rpc.Entry
	if _, ok := w.account.EntriesNative[scid]; ok {
		entries = w.account.EntriesNative[scid]
	} else {

	}

	i := sort.Search(len(entries), func(j int) bool {
		return entries[j].TopoHeight >= e.TopoHeight && entries[j].TransactionPos >= e.TransactionPos && entries[j].Pos >= e.Pos
	})

	// entry already exists, we are probably rescanning/overwiting, delete anything afterwards
	if i < len(entries) && entries[i].TopoHeight == e.TopoHeight && entries[i].TransactionPos == e.TransactionPos && entries[i].Pos == e.Pos {
		entries = entries[:i]
		// x is present at data[i]
	} else {
		// x is not present in data,
		// but i is the index where it would be inserted.
	}
	entries = append(entries, e)

	if w.account.EntriesNative == nil {
		w.account.EntriesNative = map[crypto.Hash][]rpc.Entry{}
	}
	w.account.EntriesNative[scid] = entries
}

func (w *Wallet_Memory) TokenAdd(scid crypto.Hash) (err error) {
	w.Lock()
	defer w.Unlock()

	if _, ok := w.account.EntriesNative[scid]; !ok {
		w.account.EntriesNative[scid] = []rpc.Entry{}
	} else {
		return fmt.Errorf("token already added")
	}

	return nil
}

// generate keys from using random numbers
func Generate_Keys_From_Random() (user *Account, err error) {
	user = &Account{Ringsize: 16, FeesMultiplier: 2.0}
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
	user = &Account{Ringsize: 16, FeesMultiplier: 2.0}
	language, seed, err := mnemonics.Words_To_Key(words)
	if err != nil {
		return
	}

	user.SeedLanguage = language
	user.Keys = Generate_Keys_From_Seed(crypto.GetBNRed(seed))

	return
}

func Generate_Account_From_Seed(Seed *crypto.BNRed) (user *Account, err error) {
	user = &Account{Ringsize: 16, FeesMultiplier: 2.0}

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

func (account *Account) GetAddress() (addr rpc.Address) {
	addr.PublicKey = new(crypto.Point).Set(account.Keys.Public)
	return
}

// convert a user account to address
func (w *Wallet_Memory) GetAddress() (addr rpc.Address) {
	addr = w.account.GetAddress()
	addr.Mainnet = w.account.mainnet
	return addr
}

// get a random integrated address
func (w *Wallet_Memory) GetRandomIAddress8() (addr rpc.Address) {
	addr = w.GetAddress()

	// setup random 8 bytes of payment ID, it must be from non-deterministic RNG namely crypto random
	var dstport [8]byte
	rand.Read(dstport[:])

	addr.Arguments = rpc.Arguments{{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: binary.BigEndian.Uint64(dstport[:])}}

	return
}

func (w *Wallet_Memory) Get_Balance_Rescan() (mature_balance uint64, locked_balance uint64) {
	return w.Get_Balance()
}

// get the unlocked balance ( amounts which are mature and can be spent at this time )
// offline wallets may get this wrong, since they may not have latest data//
func (w *Wallet_Memory) Get_Balance_scid(scid crypto.Hash) (mature_balance uint64, locked_balance uint64) {
	return w.account.Balance[scid], 0
}

// get main balance directly
func (w *Wallet_Memory) Get_Balance() (mature_balance uint64, locked_balance uint64) {
	var scid crypto.Hash
	return w.account.Balance[scid], 0
}

// finds all inputs which have been received/spent etc
// TODO this code can be easily parallelised and need to be parallelised
// if only the availble is requested, then the wallet is very fast
// the spent tracking may make it slow ( in case of large probably million  txs )
// TODO currently we do not track POOL at all any where ( except while building tx)
// if payment_id is true, only entries with payment ids are returned
// min_height/max height represent topoheight
func (w *Wallet_Memory) Show_Transfers(scid crypto.Hash, coinbase bool, in bool, out bool, min_height, max_height uint64, sender, receiver string, dstport, srcport uint64) []rpc.Entry {
	w.Lock()
	defer w.Unlock()

	var entries []rpc.Entry

	if max_height == 0 {
		max_height = 5000000000000
	}

	all_entries := w.account.EntriesNative[scid]
	if all_entries == nil || len(all_entries) < 1 {
		return entries
	}
	for _, e := range all_entries {
		if e.Height >= min_height && e.Height <= max_height {
			if coinbase && e.Coinbase {
				entries = append(entries, e)
				continue
			}
			if in && e.Incoming && !e.Coinbase {
				entries = append(entries, e)
				continue
			}
			if out && !(e.Incoming || e.Coinbase) {
				entries = append(entries, e)
				continue
			}
		}
	}

	//we have filtered by coinbase,in,out,min_height,max_height
	// now we must filter by sernder receiver

	return entries

}

// gets all the payments  done to specific payment ID and filtered by specific block height
// we do need better rpc
func (w *Wallet_Memory) Get_Payments_Payment_ID(scid crypto.Hash, dst_port uint64, min_height uint64) (entries []rpc.Entry) {
	return w.Get_Payments_DestinationPort(scid, dst_port, min_height)
}

// gets all the payments  done to specific payment ID and filtered by specific block height
// we do need better rpc
func (w *Wallet_Memory) Get_Payments_DestinationPort(scid crypto.Hash, port uint64, min_height uint64) (entries []rpc.Entry) {
	w.Lock()
	defer w.Unlock()
	all_entries := w.account.EntriesNative[scid]
	if all_entries == nil || len(all_entries) < 1 {
		return
	}

	for _, e := range all_entries {
		if e.Height >= min_height && e.DestinationPort == port {
			entries = append(entries, e)
		}
	}

	return

}

// return all payments within a tx there can be only 1 entry
// ZERO SCID will also search in all other tokens
// NOTE: what about multiple payments
func (w *Wallet_Memory) Get_Payments_TXID(scid crypto.Hash, txid string) (crypto.Hash, rpc.Entry) {
	w.Lock()
	defer w.Unlock()

	all_entries := w.account.EntriesNative[scid]
	if (all_entries == nil || len(all_entries) < 1) && !scid.IsZero() {
		return scid, rpc.Entry{}
	}

	for _, e := range all_entries {
		if txid == e.TXID {
			return scid, e
		}
	}

	// Its zero, maybe its optional, check in all others tokens
	if scid.IsZero() {
		for scid, entries := range w.account.EntriesNative {
			// we already processed it, skip it
			if scid.IsZero() {
				continue
			}

			for _, e := range entries {
				if txid == e.TXID {
					return scid, e
				}
			}
		}
	}

	return scid, rpc.Entry{}
}

// delete most of the data and prepare for rescan
// TODO we must save tokens list and reuse, them, but will be created on-demand when using shows transfers/or rpc apis
func (w *Wallet_Memory) Clean() {
	w.Lock()
	defer w.Unlock()
	//w.account.Entries = w.account.Entries[:0]

	for k := range w.account.EntriesNative {
		delete(w.account.EntriesNative, k)
	}

	for k := range w.account.Balance {
		delete(w.account.Balance, k)
	}

	w.account.RingMembers = map[string]int64{}
	w.account.Balance_Result = w.account.Balance_Result[:0]
	w.account.Registered = false
}

// return height of wallet
func (w *Wallet_Memory) Get_Height() uint64 {
	var scid crypto.Hash
	return uint64(w.getEncryptedBalanceresult(scid).Height)
}

// return topoheight of wallet
func (w *Wallet_Memory) Get_TopoHeight() int64 {
	var scid crypto.Hash
	return w.getEncryptedBalanceresult(scid).Topoheight
}

func (w *Wallet_Memory) Get_Daemon_Height() uint64 {
	return uint64(daemon_height)
}

// return topoheight of darmon
func (w *Wallet_Memory) Get_Daemon_TopoHeight() int64 {
	return daemon_topoheight
}

func (w *Wallet_Memory) IsRegistered() bool {
	return w.account.Registered
}

func (w *Wallet_Memory) Get_Registration_TopoHeight() int64 {
	var scid crypto.Hash
	return w.getEncryptedBalanceresult(scid).Registration
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
	Daemon_Endpoint = endpoint
	return Daemon_Endpoint
}
func SetDaemonAddress(endpoint string) string {
	Daemon_Endpoint = endpoint
	return Daemon_Endpoint
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
	defer w.save_if_disk() // save wallet

	if ringsize >= 2 && ringsize <= 128 { //reasonable limits for mixin, atleastt for now, network should bump it to 256 on next HF

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
	defer w.save_if_disk() // save wallet
	if x < 1.0 {           // fee cannot be less than 1.0, base fees
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
	defer w.save_if_disk() // save wallet

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

// Ability to set save frequency to lower disc write frequency
// a negative set value, will return existing value,
// 0 is a valid value and is set to default
// this is supposed to be used in very specific/special conditions and generally need not be changed
func (w *Wallet_Memory) SetSaveDuration(saveevery time.Duration) (old time.Duration) {
	old = w.account.SaveChangesEvery
	if saveevery >= 0 {
		if saveevery.Seconds() > 3600 {
			saveevery = 3600 * time.Second
		}
		w.account.SaveChangesEvery = saveevery
		defer w.save_if_disk() // save wallet now so as settting become permanent
	}
	return
}

// Ability to set wallet scanning
// a negative set value, will return existing value,
// 0 is a valid value and is set to default and will scan entire history
// this is supposed to be used in very specific/special conditions and generally need not be changed
func (w *Wallet_Memory) SetTrackRecentBlocks(recency int64) (old int64) {
	old = w.account.TrackRecentBlocks
	if recency >= 0 {
		w.account.TrackRecentBlocks = recency
		defer w.save_if_disk() // save wallet now so as settting become permanent
	}
	return
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

// retrieve secret key for any tx we may have created
func (w *Wallet_Memory) GetTXKey(txhash string) string {
	w.Lock()
	defer w.Unlock()

	for _, entries := range w.account.EntriesNative {
		for _, e := range entries {
			if e.TXID == txhash {
				return e.Proof
			}
		}
	}

	return ""
}

// never do any division operation on money due to floating point issues
// newbies, see type the next in python interpretor "3.33-3.13"
func FormatMoney(amount uint64) string {
	return FormatMoneyPrecision(amount, 5) // default is 5 precision after floating point
}

// format money with specific precision
func FormatMoneyPrecision(amount uint64, precision int) string {
	hard_coded_decimals := new(big.Float).SetInt64(100000)
	float_amount, _, _ := big.ParseFloat(fmt.Sprintf("%d", amount), 10, 0, big.ToZero)
	result := new(big.Float)
	result.Quo(float_amount, hard_coded_decimals)
	return result.Text('f', precision) // 5 is display precision after floating point
}

// this basically does a  Schnorr Signature on random information for registration
func (w *Wallet_Memory) SignData(input []byte) []byte {
	var tmppoint bn256.G1

	tmpsecret := crypto.RandomScalar()
	tmppoint.ScalarMult(crypto.G, tmpsecret)

	serialize := []byte(fmt.Sprintf("%s%s%x", w.account.Keys.Public.G1().String(), tmppoint.String(), input))

	c := crypto.ReducedHash(serialize)
	s := new(big.Int).Mul(c, w.account.Keys.Secret.BigInt()) // basicaly scalar mul add
	s = s.Mod(s, bn256.Order)
	s = s.Add(s, tmpsecret)
	s = s.Mod(s, bn256.Order)

	p := &pem.Block{Type: "DERO SIGNED MESSAGE"}
	p.Headers = map[string]string{}
	p.Headers["Address"] = w.GetAddress().String()
	p.Headers["C"] = fmt.Sprintf("%x", c)
	p.Headers["S"] = fmt.Sprintf("%x", s)
	p.Bytes = input

	return pem.EncodeToMemory(p)
}

func (w *Wallet_Memory) CheckSignature(input []byte) (signer *rpc.Address, message []byte, err error) {
	p, _ := pem.Decode(input)
	if p == nil {
		err = fmt.Errorf("Unknown format")
		return
	}

	astr := p.Headers["Address"]
	cstr := p.Headers["C"]
	sstr := p.Headers["S"]

	addr, err := rpc.NewAddress(astr)
	if err != nil {
		return
	}

	c, ok := new(big.Int).SetString(cstr, 16)
	if !ok {
		err = fmt.Errorf("Unknown C format")
		return
	}

	s, ok := new(big.Int).SetString(sstr, 16)
	if !ok {
		err = fmt.Errorf("Unknown S format")
		return
	}

	tmppoint := new(bn256.G1).Add(new(bn256.G1).ScalarMult(crypto.G, s), new(bn256.G1).ScalarMult(addr.PublicKey.G1(), new(big.Int).Neg(c)))
	serialize := []byte(fmt.Sprintf("%s%s%x", addr.PublicKey.G1().String(), tmppoint.String(), p.Bytes))

	c_calculated := crypto.ReducedHash(serialize)
	if c.String() != c_calculated.String() {
		err = fmt.Errorf("signature mismatch")
		return
	}

	signer = addr
	message = p.Bytes
	return
}

// this basically does a  Schnorr Signature on random information for registration
// NOTE: this function brings entire file to RAM 2 times, this could be removed by refactoring this function
// NOTE: a similar function which wraps data already exists in wallet.go just above this function
func (w *Wallet_Memory) SignFile(filename string) error {
	var tmppoint bn256.G1

	tmpsecret := crypto.RandomScalar()
	tmppoint.ScalarMult(crypto.G, tmpsecret)

	input, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	serialize := []byte(fmt.Sprintf("%s%s%x", w.account.Keys.Public.G1().String(), tmppoint.String(), input))

	c := crypto.ReducedHash(serialize)
	s := new(big.Int).Mul(c, w.account.Keys.Secret.BigInt()) // basicaly scalar mul add
	s = s.Mod(s, bn256.Order)
	s = s.Add(s, tmpsecret)
	s = s.Mod(s, bn256.Order)

	p := &pem.Block{Type: "DERO SIGNED MESSAGE"}
	p.Headers = map[string]string{}
	p.Headers["Address"] = w.GetAddress().String()
	p.Headers["C"] = fmt.Sprintf("%x", c)
	p.Headers["S"] = fmt.Sprintf("%x", s)

	return os.WriteFile(filename+".signed", pem.EncodeToMemory(p), 0600)
}

func (w *Wallet_Memory) CheckFileSignature(filename string) (signer *rpc.Address, err error) {

	input, err := os.ReadFile(filename + ".signed")
	if err != nil {
		return
	}

	p, _ := pem.Decode(input)
	if p == nil {
		err = fmt.Errorf("Unknown format")
		return
	}

	astr := p.Headers["Address"]
	cstr := p.Headers["C"]
	sstr := p.Headers["S"]

	addr, err := rpc.NewAddress(astr)
	if err != nil {
		return
	}

	c, ok := new(big.Int).SetString(cstr, 16)
	if !ok {
		err = fmt.Errorf("Unknown C format")
		return
	}

	s, ok := new(big.Int).SetString(sstr, 16)
	if !ok {
		err = fmt.Errorf("Unknown S format")
		return
	}

	tmppoint := new(bn256.G1).Add(new(bn256.G1).ScalarMult(crypto.G, s), new(bn256.G1).ScalarMult(addr.PublicKey.G1(), new(big.Int).Neg(c)))

	input_data, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	serialize := []byte(fmt.Sprintf("%s%s%x", addr.PublicKey.G1().String(), tmppoint.String(), input_data))

	c_calculated := crypto.ReducedHash(serialize)
	if c.String() != c_calculated.String() {
		err = fmt.Errorf("signature mismatch")
		return
	}

	signer = addr
	return
}
