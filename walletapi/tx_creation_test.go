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

import "io"
import "os"
import "fmt"
import "time"
import "testing"
import "bytes"

//import "crypto/rand"
import "path/filepath"

//import "encoding/hex"
//import "encoding/binary"
//import "runtime/pprof"

import "github.com/docopt/docopt-go"

import derodrpc "github.com/deroproject/derohe/cmd/derod/rpc"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/blockchain"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/cryptography/bn256"

func init() {
	globals.InitializeLog(io.Discard, io.Discard)
	GenerateProoffuncptr = generate_proof_trampoline
}

var command_line string = `derod 
DERO : A secure, private blockchain with smart-contracts

Usage:
  derod [--help] [--testnet] [--debug]  [--data-dir=<directory>] [--rpc-bind=<127.0.0.1:9999>] 
  derod -h | --help
  derod --version

Options:
  -h --help     Show this screen.
  --testnet  	Run in testnet mode.
  --debug       Debug mode enabled, print log messages
  --data-dir=<directory>    Store blockchain data at this location
  --rpc-bind=<127.0.0.1:9999>    RPC listens on this ip:port
  --node-tag=<unique name>	Unique name of node, visible to everyone
 
  `

const rpcport = "127.0.0.1:26000"

var tmpdirectory = "/tmp/dsimulatorwalletapi"

// start a chain in simulator mode
func simulator_chain_start() (*blockchain.Blockchain, *derodrpc.RPCServer, map[string]interface{}) {
	var err error
	params := map[string]interface{}{}
	params["--simulator"] = true

	parser := &docopt.Parser{
		HelpHandler:  docopt.PrintHelpOnly,
		OptionsFirst: true,
	}

	globals.Arguments, err = parser.ParseArgs(command_line, []string{"--data-dir", tmpdirectory, "--rpc-bind", rpcport}, config.Version.String())
	if err != nil {
		//log.Fatalf("Error while parsing options err: %s\n", err)
		return nil, nil, nil
	}

	os.RemoveAll(tmpdirectory)
	globals.Initialize() // setup network and proxy

	chain, err := blockchain.Blockchain_Start(params)

	if err != nil {
		panic(err)
		//return nil, nil, nil
	}

	params["chain"] = chain

	rpcserver, _ := derodrpc.RPCServer_Start(params)
	return chain, rpcserver, params
}

func simulator_chain_mineblock(chain *blockchain.Blockchain, miner_address rpc.Address, t *testing.T) {
	cbl, _, err := chain.Create_new_miner_block(miner_address)

	if err != nil {
		t.Fatalf("error creating miner block %s", err)
	}

	reg_count := 0
	normal_count := 0
	for i := range cbl.Txs {
		if cbl.Txs[i].IsRegistration() {
			reg_count++
		}
		if !cbl.Txs[i].IsRegistration() {
			normal_count++
		}

	}

	mbl := blockchain.ConvertBlockToMiniblock(*cbl.Bl, miner_address)

	cbl.Bl.MiniBlocks = append(cbl.Bl.MiniBlocks, mbl)

	t.Logf("to be mined block  height %d txs %d  reg %d normal %d mempool %d\n", cbl.Bl.Height, len(cbl.Txs), reg_count, normal_count, len(chain.Mempool.Mempool_List_TX_SortedInfo()))

	err, _ = chain.Add_Complete_Block(cbl)
	if err != nil {
		t.Fatalf("error adding block %s", err)
	}

}

func simulator_chain_stop(chain *blockchain.Blockchain, rpcserver *derodrpc.RPCServer) {

	rpcserver.RPCServer_Stop()

	chain.Shutdown() // shutdown chain subsysem
}

// this will test that the keys are placed properly and thus can be decoded by recievers
func Test_Creation_TX(t *testing.T) {

	time.Sleep(time.Millisecond)

	Initialize_LookupTable(1, 1<<17)

	wsrc_temp_db := filepath.Join(os.TempDir(), "dero_temporary_test_wallet_src.db")
	wdst_temp_db := filepath.Join(os.TempDir(), "dero_temporary_test_wallet_dst.db")

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

	chain, rpcserver, params := simulator_chain_start()
	defer simulator_chain_stop(chain, rpcserver)
	_ = params

	globals.Arguments["--daemon-address"] = rpcport

	go Keep_Connectivity()

	t.Logf("src %s\n", wsrc.GetAddress())
	t.Logf("dst %s\n", wdst.GetAddress())

	if err := chain.Add_TX_To_Pool(wsrc.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add regtx to pool err %s", err)
	}
	if err := chain.Add_TX_To_Pool(wdst.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add regtx to pool err %s", err)
	}

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip

	wgenesis.SetDaemonAddress(rpcport)
	wsrc.SetDaemonAddress(rpcport)
	wdst.SetDaemonAddress(rpcport)
	wgenesis.SetOnlineMode()
	wsrc.SetOnlineMode()
	wdst.SetOnlineMode()

	defer os.Remove(wsrc_temp_db) // cleanup after test
	defer os.Remove(wdst_temp_db) // cleanup after test

	time.Sleep(time.Second)
	if err = wsrc.Sync_Wallet_Memory_With_Daemon(); err != nil {
		t.Fatalf("wallet sync error err %s chain height %d", err, chain.Get_Height())
	}

	// here we are collecting proofs for later on bennhcmarking
	for j := 2; j <= 128; j = j * 2 {
		wsrc.account.Ringsize = j
		tx, err := wsrc.TransferPayload0([]rpc.Transfer{rpc.Transfer{Destination: wdst.GetAddress().String(), Amount: 1}}, 0, false, rpc.Arguments{}, false)
		if err != nil {
			t.Fatalf("Cannot create transaction, err %s", err)
		} else {
			var s, p bytes.Buffer
			tx.Payloads[0].Statement.Serialize(&s)
			tx.Payloads[0].Proof.Serialize(&p)

			t.Logf("Ringsize=%3d  Statement Size=%4d  Proof Size=%5d     Total TX size=%5d  bytes\n", j, len(s.Bytes()), len(p.Bytes()), len(tx.Serialize()))
			tx_maps[j] = tx
		}
	}

	wsrc.account.Ringsize = 2
	wdst.account.Ringsize = 2

	// accounts are reversed
	wdst.Sync_Wallet_Memory_With_Daemon()
	reverse_tx, err := wdst.TransferPayload0([]rpc.Transfer{rpc.Transfer{Destination: wsrc.GetAddress().String(), Amount: 1}}, 0, false, rpc.Arguments{}, false)
	if err != nil {
		t.Fatalf("Cannot create transaction, err %s", err)
	}
	var reverse_dtx transaction.Transaction
	reverse_dtx.Deserialize(reverse_tx.Serialize())

	//	fmt.Printf("balance src %v\n", wsrc.account.Balance_Mature)
	//	fmt.Printf("balance wdst1 %v ringsize %d\n", wdst.account.Balance_Mature, wdst.account.Ringsize)
	//	fmt.Printf("balance wdst2 %v\n", wdst2.account.Balance_Mature)

	pre_transfer_src_balance := wsrc.account.Balance_Mature
	pre_transfer_dst_balance := wdst.account.Balance_Mature

	tx, err := wsrc.TransferPayload0([]rpc.Transfer{rpc.Transfer{Destination: wdst.GetAddress().String(), Amount: 1}}, 0, false, rpc.Arguments{}, false)
	if err != nil {
		t.Fatalf("Cannot create transaction, err %s", err)
	} else {

	}

	var dtx transaction.Transaction
	dtx.Deserialize(tx.Serialize())

	if err := chain.Add_TX_To_Pool(&dtx); err != nil {
		t.Fatalf("Cannot add transfer tx  to pool err %s", err)
	}

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip, this is block at height 2
	wgenesis.Sync_Wallet_Memory_With_Daemon()
	wsrc.Sync_Wallet_Memory_With_Daemon()
	wdst.Sync_Wallet_Memory_With_Daemon()

	var zerohash crypto.Hash
	if _, nonce, _, _, _ := wsrc.GetEncryptedBalanceAtTopoHeight(zerohash, 2, wsrc.GetAddress().String()); nonce != 2 {
		t.Fatalf("nonce not valid. please dig. expected 2 actual %d", nonce)
	}
	if _, nonce, _, _, _ := wsrc.GetEncryptedBalanceAtTopoHeight(zerohash, 2, wdst.GetAddress().String()); nonce != 0 {
		t.Fatalf("nonce not valid. please dig. expected 0 actual %d", nonce)
	}

	post_transfer_src_balance := wsrc.account.Balance_Mature
	post_transfer_dst_balance := wdst.account.Balance_Mature

	if post_transfer_dst_balance-pre_transfer_dst_balance != 1 {
		t.Fatalf("transfer failed.Invalid balance")
	}
	//fmt.Printf("balance src %v\n", wsrc.account.Balance_Mature)
	//fmt.Printf("balance wdst1 %v ringsize %d\n", wdst.account.Balance_Mature, wdst.account.Ringsize)
	//fmt.Printf("balance wdst2 %v\n", wdst2.account.Balance_Mature)

	// now simulate a tx which has been created earlier but being mined earlier

	var tx_set []*transaction.Transaction

	for i := 0; i < 6; i++ {
		tx, err := wsrc.TransferPayload0([]rpc.Transfer{rpc.Transfer{Destination: wdst.GetAddress().String(), Amount: 1}}, 0, false, rpc.Arguments{}, false)
		if err != nil {
			t.Fatalf("Cannot create transaction, err %s", err)
		} else {
			tx_set = append(tx_set, tx)
			simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
			wdst.Sync_Wallet_Memory_With_Daemon()
		}
	}

	// now we are going to issue tx, second tx, while keeping first for test
	{
		var dtx transaction.Transaction
		dtx.Deserialize(tx_set[4].Serialize())
		if err := chain.Add_TX_To_Pool(&dtx); err != nil {
			t.Fatalf("Cannot add transfer tx  to pool err %s", err)
		}
		simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
		wdst.Sync_Wallet_Memory_With_Daemon()
	}

	//	fmt.Printf("balance src %v\n", wsrc.account.Balance_Mature)
	//fmt.Printf("balance wdst1 %v ringsize %d\n", wdst.account.Balance_Mature, wdst.account.Ringsize)
	//fmt.Printf("balance wdst2 %v\n", wdst2.account.Balance_Mature)

	for i := range tx_set { // this tx cannot go through
		var dtx transaction.Transaction
		dtx.Deserialize(tx_set[i].Serialize())
		if err := chain.Add_TX_To_Pool(&dtx); err == nil {
			t.Fatalf("This tx should NOT be added to pool %d", i)
		} else {

		}

		//wdst.Sync_Wallet_Memory_With_Daemon()
	}

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip
	wgenesis.Sync_Wallet_Memory_With_Daemon()
	wsrc.Sync_Wallet_Memory_With_Daemon()
	wdst.Sync_Wallet_Memory_With_Daemon()

	post_transfer_src_balance = wsrc.account.Balance_Mature
	post_transfer_dst_balance = wdst.account.Balance_Mature

	if post_transfer_dst_balance-pre_transfer_dst_balance != 2 {
		t.Fatalf("transfer failed.Invalid balance")
	}

	_ = pre_transfer_src_balance
	_ = post_transfer_src_balance

	post_transfer_src_balance = wsrc.account.Balance_Mature
	post_transfer_dst_balance = wdst.account.Balance_Mature

	t.Logf("src pre %d   post %d", pre_transfer_src_balance, post_transfer_src_balance)
	t.Logf("dst pre %d   post %d", pre_transfer_dst_balance, post_transfer_dst_balance)

	//now we will issue reverse tx, which sends back

	if err := chain.Add_TX_To_Pool(&reverse_dtx); err != nil {
		t.Fatalf("This tx should be added to pool err %s", err)
	}
	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t) // mine a block at tip

	wgenesis.Sync_Wallet_Memory_With_Daemon()
	wsrc.Sync_Wallet_Memory_With_Daemon()
	wdst.Sync_Wallet_Memory_With_Daemon()

	if _, nonce, _, _, _ := wsrc.GetEncryptedBalanceAtTopoHeight(zerohash, 11, wsrc.GetAddress().String()); nonce != 9 {
		t.Fatalf("nonce not valid. please dig. expected 9 actual %d", nonce)
	}
	if _, nonce, _, _, _ := wsrc.GetEncryptedBalanceAtTopoHeight(zerohash, 11, wdst.GetAddress().String()); nonce != 11 {
		t.Fatalf("nonce not valid. please dig. expected 11 actual %d", nonce)
	}

	post_transfer_src_balance = wsrc.account.Balance_Mature
	post_transfer_dst_balance = wdst.account.Balance_Mature

	t.Logf("src pre %d   post %d", pre_transfer_src_balance, post_transfer_src_balance)
	t.Logf("dst pre %d   post %d", pre_transfer_dst_balance, post_transfer_dst_balance)
	// we sent 1+1 from src
	// we sent 1 from dst to src
	if pre_transfer_src_balance-post_transfer_src_balance != 1+dtx.Fees()+reverse_dtx.Fees() {
		t.Fatalf("transfer failed.Invalid balance expected %d actual %d", 1, pre_transfer_src_balance-(post_transfer_src_balance+dtx.Fees()+reverse_dtx.Fees()))
	}

	//	fmt.Printf("balance src %v\n", wsrc.account.Balance_Mature)
	//fmt.Printf("balance wdst1 %v ringsize %d\n", wdst.account.Balance_Mature, wdst.account.Ringsize)
	//fmt.Printf("balance wdst2 %v\n", wdst2.account.Balance_Mature)

}

// since result contains random number we cannot verify output proof here
type Proof_Arguments struct {
	scid       crypto.Hash
	scid_index int
	s          *crypto.Statement
	witness    *crypto.Witness
	u          *bn256.G1
	txid       crypto.Hash
	burn_value uint64
}

var proofs_maps = map[int]*Proof_Arguments{}
var tx_maps = map[int]*transaction.Transaction{}

func generate_proof_trampoline(scid crypto.Hash, scid_index int, s *crypto.Statement, witness *crypto.Witness, u *bn256.G1, txid crypto.Hash, burn_value uint64) *crypto.Proof {
	a := &Proof_Arguments{scid: scid, scid_index: scid_index, s: s, witness: witness, u: u, txid: txid, burn_value: burn_value}
	ringsize := len(s.Publickeylist)

	if _, ok := proofs_maps[ringsize]; !ok {
		proofs_maps[ringsize] = a
	}
	return crypto.GenerateProof(scid, scid_index, s, witness, u, txid, burn_value)
}

func Benchmark_TX_Proof_Generation_2_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 2)
}
func Benchmark_TX_Proof_Generation_4_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 4)
}
func Benchmark_TX_Proof_Generation_8_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 8)
}
func Benchmark_TX_Proof_Generation_16_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 16)
}
func Benchmark_TX_Proof_Generation_32_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 32)
}
func Benchmark_TX_Proof_Generation_64_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 64)
}
func Benchmark_TX_Proof_Generation_128_ring(b *testing.B) {
	benchmark_TX_Proof_Generation(b, 128)
}

func benchmark_TX_Proof_Generation(b *testing.B, ringsize int) {
	args, ok := proofs_maps[ringsize]
	if !ok {
		b.Logf("test not availble for ringsize %d", ringsize)
		return
	}
	for n := 0; n < b.N; n++ {
		crypto.GenerateProof(args.scid, args.scid_index, args.s, args.witness, args.u, args.txid, args.burn_value)
	}
}

func Benchmark_TX_Proof_Verification_2_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 2)
}
func Benchmark_TX_Proof_Verification_4_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 4)
}
func Benchmark_TX_Proof_Verification_8_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 8)
}
func Benchmark_TX_Proof_Verification_16_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 16)
}
func Benchmark_TX_Proof_Verification_32_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 32)
}
func Benchmark_TX_Proof_Verification_64_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 64)
}
func Benchmark_TX_Proof_Verification_128_ring(b *testing.B) {
	benchmark_TX_Proof_Verification(b, 128)
}
func benchmark_TX_Proof_Verification(b *testing.B, ringsize int) {
	tx, ok := tx_maps[ringsize]
	if !ok {
		b.Logf("test not availble for ringsize %d", ringsize)
		return
	}
	for n := 0; n < b.N; n++ {

		scid_map := map[crypto.Hash]int{}

		for t := range tx.Payloads {
			index := scid_map[tx.Payloads[t].SCID]
			if !tx.Payloads[t].Proof.Verify(tx.Payloads[t].SCID, index, &tx.Payloads[t].Statement, tx.GetHash(), tx.Payloads[t].BurnValue) {
				b.Fatalf("TX verificat1ion failed, did u try sending more than you have !!!!!!!!!!\n")
			}
			scid_map[tx.Payloads[t].SCID] = scid_map[tx.Payloads[t].SCID] + 1 // increment scid counter

		}
	}
}
