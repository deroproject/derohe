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
import "time"
import "crypto/rand"
import "crypto/sha1"
import "sync"
import "runtime"

//import "strings"
//import "math/big"
//import "encoding/hex"
import "encoding/json"

import "github.com/blang/semver/v4"
import "golang.org/x/crypto/pbkdf2" // // used to encrypt master password ( so user can change his password anytime)

import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/walletapi/mnemonics"

// address book will have random number based entries

// see this https://godoc.org/golang.org/x/crypto/pbkdf2
type KDF struct {
	Hashfunction string `json:"hash"` //"SHA1" currently only sha1 is supported
	Keylen       int    `json:"keylen"`
	Iterations   int    `json:"iterations"`
	Salt         []byte `json:"salt"`
}

// this is stored in disk in encrypted form
type Wallet_Memory struct {
	Version semver.Version `json:"version"` // database version
	Secret  []byte         `json:"secret"`  // actual unlocker to the DB, depends on password from user, stored encrypted
	// secret key used to encrypt all DB data ( both keys and values )
	// this is always in encrypted form

	KDF KDF `json:"kdf"`

	account           *Account //`json:"-"` // not serialized, we store an encrypted version  // keys, seed language etc settings
	Account_Encrypted []byte   `json:"account_encrypted"`

	pbkdf2_password []byte // used to encrypt metadata on updates
	master_password []byte // single password which never changes

	Daemon_Endpoint         string `json:"-"` // endpoint used to communicate with daemon
	Merkle_Balance_TreeHash string `json:"-"` // current balance tree state

	wallet_online_mode bool // set whether the mode is online or offline
	// an offline wallet can be converted to online mode, calling.
	// SetOffline() and vice versa using SetOnline
	// used to create transaction with this fee rate,
	//if this is lower than network, then created transaction will be rejected by network
	dynamic_fees_per_kb uint64
	Quit                chan bool `json:"-"` // channel to quit any processing go routines

	db_memory   []byte       // all data is stored here
	wallet_disk *Wallet_Disk // a loopback pointer for some operations

	id string // first 8 bytes of wallet address , to put into logs to identify different wallets in case many are active

	Error error `json:"-"`

	transfer_mutex sync.Mutex // to avoid races within the transfer
	//sync.Mutex  // used to syncronise access
	sync.RWMutex

	sync_in_progress sync.Mutex // whether sync is in progress
}

// when smart contracts are implemented, each will have it's own universe to track and maintain transactions

// this file implements the encrypted data store at rest
func Create_Encrypted_Wallet_Memory(password string, seed *crypto.BNRed) (w *Wallet_Memory, err error) {
	w = &Wallet_Memory{}
	w.Version = config.Version

	if err != nil {
		return
	}

	// generate account keys
	w.account, err = Generate_Account_From_Seed(seed)
	if err != nil {
		return
	}

	// generate a 64 byte key to be used as master Key
	w.master_password = make([]byte, 32, 32)
	_, err = rand.Read(w.master_password)
	if err != nil {
		return
	}

	err = w.Set_Encrypted_Wallet_Password(password) // lock the db with the password

	w.Quit = make(chan bool)

	w.id = string((w.account.GetAddress().String())[:8]) // set unique id for logs

	var scid crypto.Hash
	w.account.Balance = map[crypto.Hash]uint64{}

	d := rpc.GetEncryptedBalance_Result{SCID: scid, Registration: -1}
	w.setEncryptedBalanceresult(scid, d)

	return
}

// create an encrypted wallet using electrum recovery words
func Create_Encrypted_Wallet_From_Recovery_Words_Memory(password string, electrum_seed string) (w *Wallet_Memory, err error) {

	language, seed, err := mnemonics.Words_To_Key(electrum_seed)
	if err != nil {
		return
	}
	w, err = Create_Encrypted_Wallet_Memory(password, crypto.GetBNRed(seed))

	if err != nil {
		return
	}

	w.account.SeedLanguage = language
	return
}

// create an encrypted wallet using using random data
func Create_Encrypted_Wallet_Random_Memory(password string) (w *Wallet_Memory, err error) {
	w, err = Create_Encrypted_Wallet_Memory(password, crypto.RandomScalarBNRed())

	if err != nil {
		return
	}
	// TODO setup seed language, default is already english
	return
}

// wallet must already be open
func (w *Wallet_Memory) Set_Encrypted_Wallet_Password(password string) (err error) {

	if w == nil {
		return
	}
	w.Lock()

	// set up KDF structure
	w.KDF.Salt = make([]byte, 32, 32)
	_, err = rand.Read(w.KDF.Salt)
	if err != nil {
		w.Unlock()
		return
	}
	w.KDF.Keylen = 32
	w.KDF.Iterations = 262144
	w.KDF.Hashfunction = "SHA1"

	if runtime.GOOS == "js" {
		w.KDF.Iterations = 32768
	}

	if globals.IsSimulator() {
		w.KDF.Iterations = 10
	}

	// lets generate the encrypted password

	w.pbkdf2_password = Generate_Key(w.KDF, password)

	w.Unlock()
	w.Save_Wallet() // save wallet data

	return
}

func Open_Encrypted_Wallet_Memory(password string, filedata []byte) (w *Wallet_Memory, err error) {
	w = &Wallet_Memory{}

	//fmt.Printf("v %+v\n",string(v)) // DO NOT dump account keys

	// deserialize json data
	err = json.Unmarshal(filedata, &w)
	if err != nil {
		return
	}

	w.Quit = make(chan bool)
	// todo make any routines necessary, such as sync etc

	// try to deseal password and store it
	w.pbkdf2_password = Generate_Key(w.KDF, password)

	// try to decrypt the master password with the pbkdf2
	w.master_password, err = DecryptWithKey(w.pbkdf2_password, w.Secret) // decrypt the master key
	if err != nil {
		err = fmt.Errorf("Invalid Password")
		w = nil
		return
	}

	// password has been  found, open the account

	account_bytes, err := w.Decrypt(w.Account_Encrypted)
	if err != nil {
		//rlog.Errorf("err opening account err: %s ", err)
		err = fmt.Errorf("probably Invalid Password")
		w = nil
		return
	}

	w.account = &Account{} // allocate a new instance
	err = json.Unmarshal(account_bytes, w.account)
	if err != nil {
		return
	}
	var scid crypto.Hash
	d := rpc.GetEncryptedBalance_Result{SCID: scid, Registration: -1}
	w.setEncryptedBalanceresult(scid, d)
	if w.account.Balance == nil {
		w.account.Balance = map[crypto.Hash]uint64{}
	}

	return

}

// check whether the already opened wallet can use this password
func (w *Wallet_Memory) Check_Password(password string) bool {
	w.Lock()
	defer w.Unlock()
	if w == nil {
		return false
	}

	pbkdf2_password := Generate_Key(w.KDF, password)

	// TODO we can compare pbkdf2_password & w.pbkdf2_password, if they are equal password is vaid

	// try to decrypt the master password with the pbkdf2
	_, err := DecryptWithKey(pbkdf2_password, w.Secret) // decrypt the master key

	if err == nil {
		return true
	}
	return false

}

// save updated copy of wallet
func (w *Wallet_Memory) Save_Wallet() (err error) {
	w.Lock()
	defer w.Unlock()
	if w == nil {
		return
	}

	// encrypted the master password with the pbkdf2
	w.Secret, err = EncryptWithKey(w.pbkdf2_password[:], w.master_password) // encrypt the master key
	if err != nil {
		return
	}

	// encrypt the account

	account_serialized, err := json.Marshal(w.account)
	if err != nil {
		return
	}

	//  fmt.Printf("account serialized %s\n", string(account_serialized))
	//  fmt.Printf("account serialized full %+v  %s\n", w.account , w.account.Keys.Secret.Text(16))

	w.Account_Encrypted, err = w.Encrypt(account_serialized)
	if err != nil {
		return
	}

	// json marshal wallet data struct, serialize it, encrypt it and store it
	serialized, err := json.Marshal(&w)
	if err != nil {
		return
	}
	//fmt.Printf("Serialized  %+v\n",serialized)

	w.db_memory = serialized
	return
}

// get encrypted wallet
func (w *Wallet_Memory) Get_Encrypted_Wallet() []byte {
	if err := w.Save_Wallet(); err == nil {
		return w.db_memory
	}
	return []byte{}
}

// close the wallet
// note that w is still valid and can be used to obtaine encrypted copy of data
func (w *Wallet_Memory) Close_Encrypted_Wallet() {
	time.Sleep(time.Second) // give goroutines some time to quit
	w.Save_Wallet()
}

// generate key from password
func Generate_Key(k KDF, password string) (key []byte) {
	switch k.Hashfunction {
	case "SHA1":
		return pbkdf2.Key([]byte(password), k.Salt, k.Iterations, k.Keylen, sha1.New)

	default:
		return pbkdf2.Key([]byte(password), k.Salt, k.Iterations, k.Keylen, sha1.New)
	}
}

func (w *Wallet_Memory) GetAccount() *Account {
	if w == nil {
		return nil
	}
	return w.account
}

func (w *Wallet_Memory) save_if_disk() {
	if w == nil || w.wallet_disk == nil {
		return
	}
	if runtime.GOARCH != "wasm" {
		w.wallet_disk.Save_Wallet()
	}
}
