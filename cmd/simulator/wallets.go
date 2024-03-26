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

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/deroproject/derohe/blockchain"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/transaction"
	"github.com/deroproject/derohe/walletapi"
	"github.com/deroproject/derohe/walletapi/rpcserver"
	"github.com/deroproject/derohe/walletapi/xswd"
) //import "math/big"

const WALLET_PASSWORD = ""

var genesis_seed = "0206a2fca2d2da068dfa8f792ef190a352d656910895f6c541d54877fca95a77"

var wallets_seeds = []string{
	"171eeaa899e360bf1a8ada7627aaea9fdad7992463581d935a8838f16b1ff51a",
	"193faf64d79e9feca5fce8b992b4bb59b86c50f491e2dc475522764ca6666b6b",
	"2e49383ac5c938c268921666bccfcb5f0c4d43cd3ed125c6c9e72fc5620bc79b",
	"1c8ee58431e21d1ef022ccf1f53fec36f5e5851d662a3dd96ced3fc155445120",
	"19182604625563f3ff913bb8fb53b0ade2e0271ca71926edb98c8e39f057d557",
	"2a3beb8a57baa096512e85902bb5f1833f1f37e79f75227bbf57c4687bfbb002",
	"055e43ebff20efff612ba6f8128caf990f2bf89aeea91584e63179b9d43cd3ab",
	"2ccb7fc12e867796dd96e246aceff3fea1fdf78a28253c583017350034c31c81",
	"279533d87cc4c637bf853e630480da4ee9d4390a282270d340eac52a391fd83d",
	"03bae8b71519fe8ac3137a3c77d2b6a164672c8691f67bd97548cb6c6f868c67",
	"2b9022d0c5ee922439b0d67864faeced65ebce5f35d26e0ee0746554d395eb88",
	"1a63d5cf9955e8f3d6cecde4c9ecbd538089e608741019397824dc6a2e0bfcc1",
	"10900d25e7dc0cec35fcca9161831a02cb7ed513800368529ba8944eeca6e949",
	"2af6630905d73ee40864bd48339f297908a0731a6c4c6fa0a27ea574ac4e4733",
	"2ac9a8984c988fcb54b261d15bc90b5961d673bffa5ff41c8250c7e262cbd606",
	"040572cec23e6df4f686192b776c197a50591836a3dd02ba2e4a7b7474382ccd",
	"2b2b029cfbc5d08b5d661e6fa444102d387780bec088f4dd41a4a537bf9762af",
	"1812298da90ded6457b2a20fd52d09f639584fb470c715617db13959927be7f8",
	"1eee334e1f533aa1ac018124cf3d5efa20e52f54b05e475f6f2cff3476b4a92f",
	"2c34e7978ce249aebed33e14cdd5177921ecd78fbe58d33bbec21f22b80af7a5",
	"083e7fe96e8415ea119ec6c4d0ebe233e86b53bd4e2f7598505317efc23ae34b",
	"0fd7f8db0ed6cbe3bf300258619d8d4a2ff8132ef3c896f6e3fa65a6c92bdf9a",
}

var genesis_wallet *walletapi.Wallet_Disk
var wallets []*walletapi.Wallet_Disk
var wallets_rpcservers []*rpcserver.RPCServer
var wallets_xswdservers []*xswd.XSWD

func create_wallet(name string, seed string) (wallet *walletapi.Wallet_Disk) {

	seed_raw, err := hex.DecodeString(seed) // hex decode
	if len(seed) >= 65 || err != nil {      //sanity check
		logger.Error(err, "Seed must be less than 66 chars hexadecimal chars")
		return nil
	}

	filename := filepath.Join(globals.GetDataDirectory(), name)

	os.Remove(filename)

	wallet, err = walletapi.Create_Encrypted_Wallet(filename, WALLET_PASSWORD, new(crypto.BNRed).SetBytes(seed_raw))
	if err != nil {
		logger.Error(err, "Error while recovering wallet using seed key")
		return nil
	}
	wallet.SetNetwork(!globals.Arguments["--testnet"].(bool))
	wallet.Save_Wallet()
	return

}

func create_genesis_wallet() {
	genesis_wallet = create_wallet("genesis", genesis_seed)
	fix_startup() // fixup genesis
}

func fix_startup() {
	genesis_tx := transaction.Transaction{Transaction_Prefix: transaction.Transaction_Prefix{Version: 1, Value: 112345}}
	copy(genesis_tx.MinerAddress[:], genesis_wallet.GetAddress().PublicKey.EncodeCompressed())
	config.Testnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())
	config.Mainnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())

	genesis_block := blockchain.Generate_Genesis_Block()
	config.Testnet.Genesis_Block_Hash = genesis_block.GetHash()
	config.Mainnet.Genesis_Block_Hash = genesis_block.GetHash()

}

// genesis wallet already exists, register other wallet by sending registratuin tx, then mining them
func register_wallets(chain *blockchain.Blockchain) {
	for i := range wallets_seeds {
		wallets = append(wallets, create_wallet(fmt.Sprintf("wallet_%d.db", i), wallets_seeds[i]))
	}
	for i := range wallets { // first register wallets
		err := chain.Add_TX_To_Pool(wallets[i].GetRegistrationTX())
		if err != nil {
			logger.Error(err, "Cannot add regtx to pool")
		}
		wallets[i].SetDaemonAddress(rpcport)
		wallets[i].SetOnlineMode() // make wallet connect to daemon

		if v, ok := globals.Arguments["--use-xswd"]; ok && v.(bool) {
			// XSWD server accept everything by default
			xswd.NewXSWDServerWithPort(wallet_ports_xswd_start+i, wallets[i], func(app *xswd.ApplicationData) bool {
				return true
			}, func(app *xswd.ApplicationData, request *jrpc2.Request) xswd.Permission {
				return xswd.Allow
			})
		}

		globals.Arguments["--rpc-bind"] = fmt.Sprintf("127.0.0.1:%d", wallet_ports_start+i)

		if r, err := rpcserver.RPCServer_Start(wallets[i], fmt.Sprintf("wallet_%d", i)); err != nil {
			logger.Error(err, "Error starting rpc server")
		} else {
			logger.Info(fmt.Sprintf("wallet %d", i), "seed", wallets_seeds[i])
			wallets_rpcservers = append(wallets_rpcservers, r)
		}

		time.Sleep(17 * time.Millisecond) // enough delay to start a go routine
	}
}

func stop_rpcs() {
	for _, r := range wallets_rpcservers {
		go r.RPCServer_Stop()
	}

	for _, r := range wallets_xswdservers {
		go r.Stop()
	}
}
