// +build !wasm

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

import "os"
import "fmt"
import "time"
import "sync"
import "io/ioutil"

import "github.com/romana/rlog"

import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/walletapi/mnemonics"

// this is stored in disk in encrypted form
type Wallet_Disk struct {
	*Wallet_Memory
	filename string
	sync.Mutex
}

// when smart contracts are implemented, each will have it's own universe to track and maintain transactions

// this file implements the encrypted data store at rest
func Create_Encrypted_Wallet(filename string, password string, seed *crypto.BNRed) (w *Wallet_Disk, err error) {

	if _, err = os.Stat(filename); err == nil {
		err = fmt.Errorf("File '%s' already exists", filename)
		return

	} else if os.IsNotExist(err) {
		// path/to/whatever does *not* exist
		// err = fmt.Errorf("path does not exists '%s'", filename)

	}

	wd := &Wallet_Disk{filename: filename}

	// generate account keys
	if wd.Wallet_Memory, err = Create_Encrypted_Wallet_Memory(password, seed); err != nil {
		return nil, err
	}

	return
}

// create an encrypted wallet using electrum recovery words
func Create_Encrypted_Wallet_From_Recovery_Words(filename string, password string, electrum_seed string) (wd *Wallet_Disk, err error) {
	wd = &Wallet_Disk{filename: filename}

	language, seed, err := mnemonics.Words_To_Key(electrum_seed)
	if err != nil {
		rlog.Errorf("err parsing recovery words %s", err)
		return
	}
	if wd.Wallet_Memory, err = Create_Encrypted_Wallet_Memory(password, crypto.GetBNRed(seed)); err != nil {
		rlog.Errorf("err creating wallet %s", err)
		return nil, err
	}

	wd.Wallet_Memory.account.SeedLanguage = language
	return
}

// create an encrypted wallet using using random data
func Create_Encrypted_Wallet_Random(filename string, password string) (wd *Wallet_Disk, err error) {
	rlog.Infof("Creating Wallet Randomly")
	wd = &Wallet_Disk{filename: filename}
	if wd.Wallet_Memory, err = Create_Encrypted_Wallet_Memory(password, crypto.RandomScalarBNRed()); err == nil {
		return wd, nil
	}

	return nil, err
}

// wallet must already be open
func (w *Wallet_Disk) Set_Encrypted_Wallet_Password(password string) (err error) {
	if w != nil {
		w.Wallet_Memory.Set_Encrypted_Wallet_Password(password)
		w.Save_Wallet() // save wallet data
	}
	return
}

func Open_Encrypted_Wallet(filename string, password string) (wd *Wallet_Disk, err error) {
	wd = &Wallet_Disk{}
	var filedata []byte

	if _, err = os.Stat(filename); os.IsNotExist(err) {
		err = fmt.Errorf("File '%s' does NOT exists", filename)
		rlog.Errorf("err opening wallet %s", err)
		return nil, err
	}

	if filedata, err = ioutil.ReadFile(filename); err != nil {
		rlog.Errorf("err reading files %s", err)
		return nil, err
	}

	if wd.Wallet_Memory, err = Open_Encrypted_Wallet_Memory(password, filedata); err != nil {
		return nil, err
	}

	return

}

// check whether the already opened wallet can use this password
func (w *Wallet_Disk) Check_Password(password string) bool {
	w.Lock()
	defer w.Unlock()

	return w.Wallet_Memory.Check_Password(password)

}

// save updated copy of wallet
func (w *Wallet_Disk) Save_Wallet() (err error) {
	if w == nil {
		return
	}
	w.Lock()
	defer w.Unlock()

	if err = w.Wallet_Memory.Save_Wallet(); err != nil {
		return
	}

	return ioutil.WriteFile(w.filename, w.Wallet_Memory.db_memory, 0600)
}

// close the wallet
func (w *Wallet_Disk) Close_Encrypted_Wallet() {
	time.Sleep(time.Second) // give goroutines some time to quit
	w.Save_Wallet()
	w.Wallet_Memory.Close_Encrypted_Wallet()
}
