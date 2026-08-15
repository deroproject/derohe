package walletapi

import "os"
import "fmt"
import "time"
import "testing"
import "path/filepath"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/blockchain"
import "github.com/deroproject/derohe/transaction"

// Test_IntegratedAddress_BuildsTX proves that sending to an integrated
// (deroi1...) address actually builds a transaction.
//
// TransferPayload0 decoded the integrated address's destination port and then
// hit an unconditional `return`, handing the caller a nil tx AND a nil err --
// a silent no-op indistinguishable from success. Callers surface this as
// "Can not send nil transaction" (-32098) or a nil dereference.
//
// FAILS on stock (unconditional return), PASSES once the return is gated on err.
func Test_IntegratedAddress_BuildsTX(t *testing.T) {

	time.Sleep(time.Millisecond)

	Initialize_LookupTable(1, 1<<17)

	wsrc_temp_db := filepath.Join(os.TempDir(), "dero_intaddr_test_src.db")
	wdst_temp_db := filepath.Join(os.TempDir(), "dero_intaddr_test_dst.db")
	wgen_temp_db := filepath.Join(os.TempDir(), "dero_intaddr_test_gen.db")

	os.Remove(wsrc_temp_db)
	os.Remove(wdst_temp_db)
	os.Remove(wgen_temp_db)

	wsrc, err := Create_Encrypted_Wallet_From_Recovery_Words(wsrc_temp_db, "QWER", "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly")
	if err != nil {
		t.Fatalf("Cannot create src wallet, err %s", err)
	}
	wdst, err := Create_Encrypted_Wallet_From_Recovery_Words(wdst_temp_db, "QWER", "Dekade Spagat Bereich Radclub Yeti Dialekt Unimog Nomade Anlage Hirte Besitz Märzluft Krabbe Nabel Halsader Chefarzt Hering tauchen Neuerung Reifen Umgang Hürde Alchimie Amnesie Reifen")
	if err != nil {
		t.Fatalf("Cannot create dst wallet, err %s", err)
	}
	wgenesis, err := Create_Encrypted_Wallet_From_Recovery_Words(wgen_temp_db, "QWER", "perfil lujo faja puma favor pedir detalle doble carbón neón paella cuarto ánimo cuento conga correr dental moneda león donar entero logro realidad acceso doble")
	if err != nil {
		t.Fatalf("Cannot create genesis wallet, err %s", err)
	}

	defer os.Remove(wsrc_temp_db)
	defer os.Remove(wdst_temp_db)
	defer os.Remove(wgen_temp_db)

	genesis_tx := transaction.Transaction{Transaction_Prefix: transaction.Transaction_Prefix{Version: 1, Value: 2012345}}
	copy(genesis_tx.MinerAddress[:], wgenesis.account.Keys.Public.EncodeCompressed())

	config.Testnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())
	config.Mainnet.Genesis_Tx = fmt.Sprintf("%x", genesis_tx.Serialize())

	genesis_block := blockchain.Generate_Genesis_Block()
	config.Testnet.Genesis_Block_Hash = genesis_block.GetHash()
	config.Mainnet.Genesis_Block_Hash = genesis_block.GetHash()

	chain, rpcserver, _ := simulator_chain_start()
	defer simulator_chain_stop(chain, rpcserver)

	globals.Arguments["--daemon-address"] = rpcport
	go Keep_Connectivity()

	if err := chain.Add_TX_To_Pool(wsrc.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add src regtx to pool err %s", err)
	}
	if err := chain.Add_TX_To_Pool(wdst.GetRegistrationTX()); err != nil {
		t.Fatalf("Cannot add dst regtx to pool err %s", err)
	}

	simulator_chain_mineblock(chain, wgenesis.GetAddress(), t)

	wgenesis.SetDaemonAddress(rpcport)
	wsrc.SetDaemonAddress(rpcport)
	wdst.SetDaemonAddress(rpcport)
	wgenesis.SetOnlineMode()
	wsrc.SetOnlineMode()
	wdst.SetOnlineMode()

	time.Sleep(time.Second)
	if err = wsrc.Sync_Wallet_Memory_With_Daemon(); err != nil {
		t.Fatalf("wallet sync error err %s chain height %d", err, chain.Get_Height())
	}

	wsrc.account.Ringsize = 2
	wdst.account.Ringsize = 2

	// build an integrated address for wdst carrying only a destination port,
	// i.e. exactly what MakeIntegratedAddress produces for a payment ID.
	intaddr := wdst.GetAddress()
	intaddr.Arguments = rpc.Arguments{
		{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: uint64(0xDEADBEEF)},
	}
	if !intaddr.IsIntegratedAddress() {
		t.Fatalf("constructed address is not integrated: %s", intaddr.String())
	}
	t.Logf("integrated destination: %s", intaddr.String())

	// control: the plain base address must build a tx
	base_tx, err := wsrc.TransferPayload0([]rpc.Transfer{{Destination: wdst.GetAddress().String(), Amount: 1}}, 0, false, rpc.Arguments{}, 0, false)
	if err != nil || base_tx == nil {
		t.Fatalf("CONTROL FAILED: plain address should build a tx (tx=%v err=%v)", base_tx, err)
	}
	t.Logf("control ok: plain address built tx %s", base_tx.GetHash())

	// the actual assertion
	tx, err := wsrc.TransferPayload0([]rpc.Transfer{{Destination: intaddr.String(), Amount: 1}}, 0, false, rpc.Arguments{}, 0, false)

	if err != nil {
		t.Fatalf("integrated address returned an error: %s", err)
	}
	if tx == nil {
		t.Fatalf("BUG REPRODUCED: integrated address returned nil tx AND nil err -- silent no-op")
	}

	t.Logf("integrated address built tx %s (%d bytes)", tx.GetHash(), len(tx.Serialize()))
}
