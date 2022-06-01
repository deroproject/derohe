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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stratumfarm/derohe/blockchain"
	"github.com/stratumfarm/derohe/config"
	"github.com/stratumfarm/derohe/globals"
	"github.com/stratumfarm/derohe/transaction"
	"github.com/stratumfarm/derohe/walletapi"
)

//import "github.com/stratumfarm/derohe/cryptography/crypto"

// this will test that the keys are placed properly and thus can be decoded by recievers
func Test_Blockchain_Deviation(t *testing.T) {

	time.Sleep(time.Millisecond)

	walletapi.Initialize_LookupTable(1, 1<<17)

	wsrc_temp_db := filepath.Join(os.TempDir(), "dero_temporary_test_wallet_src.db")
	wdst_temp_db := filepath.Join(os.TempDir(), "dero_temporary_test_wallet_dst.db")

	os.Remove(wsrc_temp_db)
	os.Remove(wdst_temp_db)

	wsrc, err := walletapi.Create_Encrypted_Wallet_From_Recovery_Words(wsrc_temp_db, "QWER", "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly")
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}

	wdst, err := walletapi.Create_Encrypted_Wallet_From_Recovery_Words(wdst_temp_db, "QWER", "Dekade Spagat Bereich Radclub Yeti Dialekt Unimog Nomade Anlage Hirte Besitz Märzluft Krabbe Nabel Halsader Chefarzt Hering tauchen Neuerung Reifen Umgang Hürde Alchimie Amnesie Reifen")
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}

	wgenesis, err := walletapi.Create_Encrypted_Wallet_From_Recovery_Words(wdst_temp_db, "QWER", "perfil lujo faja puma favor pedir detalle doble carbón neón paella cuarto ánimo cuento conga correr dental moneda león donar entero logro realidad acceso doble")
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}

	// fix genesis tx and genesis tx hash
	genesis_tx := transaction.Transaction{Transaction_Prefix: transaction.Transaction_Prefix{Version: 1, Value: 2012345}}
	copy(genesis_tx.MinerAddress[:], wgenesis.GetAddress().PublicKey.EncodeCompressed())

	config.Testnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())
	config.Mainnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())

	genesis_block := blockchain.Generate_Genesis_Block()
	config.Testnet.Genesis_Block_Hash = genesis_block.GetHash()
	config.Mainnet.Genesis_Block_Hash = genesis_block.GetHash()

	chain, rpcserver, _ := simulator_chain_start()
	defer simulator_chain_stop(chain, rpcserver)

	globals.Arguments["--daemon-address"] = rpcport_test

	t.Logf("src %s\n", wsrc.GetAddress())
	t.Logf("dst %s\n", wdst.GetAddress())

	if err := chain.Add_TX_To_Pool(wsrc.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add regtx to pool err %s", err)
	}
	if err := chain.Add_TX_To_Pool(wdst.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add regtx to pool err %s", err)
	}

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip

	defer os.Remove(wsrc_temp_db) // cleanup after test
	defer os.Remove(wdst_temp_db) // cleanup after test

	// we do not need wallets any more, start the tests

	cbl1, _, err := chain.Create_new_miner_block(wgenesis.GetAddress())
	if err != nil {
		panic(err)
	}
	cbl2, _, err := chain.Create_new_miner_block(wgenesis.GetAddress())
	if err != nil {
		panic(err)
	}
	cbl3, _, err := chain.Create_new_miner_block(wgenesis.GetAddress())
	if err != nil {
		panic(err)
	}
	_ = cbl3

	cbl1.Bl.MiniBlocks = append(cbl1.Bl.MiniBlocks, blockchain.ConvertBlockToMiniblock(*cbl1.Bl, wgenesis.GetAddress()))
	cbl2.Bl.MiniBlocks = append(cbl2.Bl.MiniBlocks, blockchain.ConvertBlockToMiniblock(*cbl2.Bl, wgenesis.GetAddress()))
	//cbl3.Bl.MiniBlocks = append(cbl3.Bl.MiniBlocks, blockchain.ConvertBlockToMiniblock(*cbl3.Bl, wgenesis.GetAddress()))

	if err, _ = chain.Add_Complete_Block(cbl1); err != nil {
		t.Fatalf("error adding block %s", err)
	}
	for i := 0; i < 4; i++ {

		cbl_next, _, err := chain.Create_new_miner_block(wgenesis.GetAddress())
		if err != nil {
			panic(err)
		}
		mbl := blockchain.ConvertBlockToMiniblock(*cbl_next.Bl, wgenesis.GetAddress())
		if mbl.PastCount != 1 {
			t.Fatalf("miniblock should have 1 tips but has %d past", mbl.PastCount)
		}
		if len(cbl_next.Bl.MiniBlocks) != i {
			t.Fatalf("Expecting %d blocks but have %d", i, len(cbl_next.Bl.MiniBlocks))
		}

		if err, ok := chain.InsertMiniBlock(mbl); !ok || err != nil {
			t.Fatalf("miniblock should be inserted")
		}

	}
	if err, _ = chain.Add_Complete_Block(cbl2); err != nil {
		t.Fatalf("error adding block %s", err)
	}

	t.Logf("chain height %d tips %+v", chain.Get_Height(), chain.Get_TIPS())

	// at this point chain has forked and we should have 2 tips
	if len(chain.Get_TIPS()) != 2 {
		t.Fatalf("cannot fork successfully, tips %d", len(chain.Get_TIPS()))
	}

	cbl, _, err := chain.Create_new_miner_block(wgenesis.GetAddress())
	if err != nil {
		panic(err)
	}

	mbl := blockchain.ConvertBlockToMiniblock(*cbl.Bl, wgenesis.GetAddress())
	if mbl.PastCount != 1 {
		t.Fatalf("miniblock should have 1 tips but has %d past", mbl.PastCount)
	}

	for i := 0; i < 4; i++ {

		cbl_next, _, err := chain.Create_new_miner_block(wgenesis.GetAddress())
		if err != nil {
			panic(err)
		}
		if cbl_next.Bl.Height != 3 {
			t.Fatalf("Height Expected %d Actuak %d", 3, cbl_next.Bl.Height)
		}
		mbl := blockchain.ConvertBlockToMiniblock(*cbl_next.Bl, wgenesis.GetAddress())

		t.Logf("mbl %+v\n", mbl)
		if i == 0 {

		}

		if len(cbl_next.Bl.MiniBlocks) != 4+i {
			t.Fatalf("Expecting %d blocks but have %d", 4+i, len(cbl_next.Bl.MiniBlocks))
		}

		precount := chain.MiniBlocks.Count()
		if err, ok := chain.InsertMiniBlock(mbl); !ok || err != nil {
			t.Fatalf("miniblock should be inserted")
		}
		if precount+1 != chain.MiniBlocks.Count() {
			t.Fatalf("miniblock count not increased.")
		}

		if tips := chain.MiniBlocks.GetAllKeys(int64(cbl_next.Bl.Height)); len(tips) != 1 {
			t.Fatalf("Tip count Expected %d Actuak %d", 1, len(tips))
		}

	}

}
