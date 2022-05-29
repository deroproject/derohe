// Copyright 2017-2018 DERO Project. All rights reserved.
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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deroproject/derohe/blockchain"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

// this will test that the keys are placed properly and thus can be decoded by recievers
func Test_Creation_TX_morecheck(t *testing.T) {

	time.Sleep(time.Millisecond)

	Initialize_LookupTable(1, 1<<17)

	wsrc_temp_db := filepath.Join(os.TempDir(), "1dero_temporary_test_wallet_src.db")
	wdst_temp_db := filepath.Join(os.TempDir(), "1dero_temporary_test_wallet_dst.db")

	os.Remove(wsrc_temp_db)
	os.Remove(wdst_temp_db)

	wsrc, err := Create_Encrypted_Wallet_From_Recovery_Words(wsrc_temp_db, "QWER", "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly")
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}

	wdst, err := Create_Encrypted_Wallet_From_Recovery_Words(wdst_temp_db, "QWER", "Dekade Spagat Bereich Radclub Yeti Dialekt Unimog Nomade Anlage Hirte Besitz Märzluft Krabbe Nabel Halsader Chefarzt Hering tauchen Neuerung Reifen Umgang Hürde Alchimie Amnesie Reifen")
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}

	wgenesis, err := Create_Encrypted_Wallet_From_Recovery_Words(wdst_temp_db, "QWER", "perfil lujo faja puma favor pedir detalle doble carbón neón paella cuarto ánimo cuento conga correr dental moneda león donar entero logro realidad acceso doble")
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}

	// fix genesis tx and genesis tx hash
	genesis_tx := transaction.Transaction{Transaction_Prefix: transaction.Transaction_Prefix{Version: 1, Value: 2012345}}
	copy(genesis_tx.MinerAddress[:], wgenesis.account.Keys.Public.EncodeCompressed())

	config.Testnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())
	config.Mainnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())

	genesis_block := blockchain.Generate_Genesis_Block()
	config.Testnet.Genesis_Block_Hash = genesis_block.GetHash()
	config.Mainnet.Genesis_Block_Hash = genesis_block.GetHash()

	chain, rpcserver, _ := simulator_chain_start()
	defer simulator_chain_stop(chain, rpcserver)

	client := NewRPCCLient(rpcport)
	go client.Keep_Connectivity()
	wsrc.SetClient(client)
	wdst.SetClient(client)
	wgenesis.SetClient(client)

	t.Logf("src %s\n", wsrc.GetAddress())
	t.Logf("dst %s\n", wdst.GetAddress())

	if err := chain.Add_TX_To_Pool(wsrc.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add regtx to pool err %s", err)
	}
	if err := chain.Add_TX_To_Pool(wdst.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add regtx to pool err %s", err)
	}

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip

	wgenesis.SetOnlineMode()
	wsrc.SetOnlineMode()
	wdst.SetOnlineMode()

	defer os.Remove(wsrc_temp_db) // cleanup after test
	defer os.Remove(wdst_temp_db) // cleanup after test

	time.Sleep(time.Second)
	if err = wsrc.Sync_Wallet_Memory_With_Daemon(); err != nil {
		t.Fatalf("wallet sync error err %s", err)
	}
	if err = wdst.Sync_Wallet_Memory_With_Daemon(); err != nil {
		t.Fatalf("wallet sync error err %s", err)
	}

	wsrc.account.Ringsize = 2
	wdst.account.Ringsize = 2

	var txs []transaction.Transaction
	for i := 0; i < 7; i++ {

		wsrc.Sync_Wallet_Memory_With_Daemon()
		wdst.Sync_Wallet_Memory_With_Daemon()

		t.Logf("Chain height %d\n", chain.Get_Height())

		tx, err := wsrc.TransferPayload0([]rpc.Transfer{{Destination: wdst.GetAddress().String(), Amount: 700000}}, 0, false, rpc.Arguments{}, 100000, false)
		if err != nil {
			t.Fatalf("Cannot create transaction, err %s", err)
		} else {

		}

		var dtx transaction.Transaction
		dtx.Deserialize(tx.Serialize())

		simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip, this is block at height 2
		wsrc.Sync_Wallet_Memory_With_Daemon()
		wdst.Sync_Wallet_Memory_With_Daemon()
		txs = append(txs, dtx)
	}

	for i := range txs {
		if err := chain.Add_TX_To_Pool(&txs[i]); err != nil {
			t.Fatalf("Cannot add transfer tx  to pool err %s", err)
		}
	}

	// close source wallet
	wsrc.Close_Encrypted_Wallet()

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip, this is block at height 2
	wgenesis.Sync_Wallet_Memory_With_Daemon()

	wdst.Sync_Wallet_Memory_With_Daemon()

	//fmt.Printf("balance src %v\n", wsrc.account.Balance_Mature)
	//fmt.Printf("balance wdst %v ringsize %d\n", wdst.account.Balance_Mature, wdst.account.Ringsize)
	//fmt.Printf("balance wdst2 %v\n", wdst2.account.Balance_Mature)

	if wdst.account.Balance_Mature != 1500000 {
		t.Fatalf("failed balance check, expected 1500000 actual %d", wdst.account.Balance_Mature)
	}
}
