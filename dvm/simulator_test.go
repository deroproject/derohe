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

package dvm

//import "fmt"
//import "reflect"
import (
	"strings"
	"testing"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
)

var sc = `/* Lottery Smart Contract Example in DVM-BASIC.  
This lottery smart contract will give lottery wins on every second try in following default contract.
	Make depost transaction to this SCID to play lottery. 
	Check https://github.com/deroproject/derohe/blob/main/guide/examples/lottery_sc_guide.md
*/



        Function Lottery() Uint64
	10  dim deposit_count,winner as Uint64
	20  LET deposit_count =  LOAD("deposit_count")+1
	25  IF DEROVALUE() == 0 THEN GOTO 110  // if deposit amount is 0, simply return
	30  STORE("depositor_address" + (deposit_count-1), SIGNER()) // store address for later on payment
	40  STORE("deposit_total", LOAD("deposit_total") + DEROVALUE() )
	50  STORE("deposit_count",deposit_count)
	60  IF LOAD("lotteryeveryXdeposit") > deposit_count THEN GOTO 110 // we will wait till X players join in
        // we are here means all players have joined in, roll the DICE, 
	70  LET winner  = RANDOM() % deposit_count // we have a winner
	80  SEND_DERO_TO_ADDRESS(LOAD("depositor_address" + winner) , LOAD("lotterygiveback")*LOAD("deposit_total")/10000)
        // Re-Initialize for another round
        90  STORE("deposit_count", 0)   //  initial players
	100 STORE("deposit_total", 0)   //  total deposit of all players
	110  RETURN 0
	End Function

	
	// This function is used to initialize parameters during install time
	Function Initialize() Uint64
	5   version("1.2.3")
	10  STORE("owner", SIGNER())   // store in DB  ["owner"] = address
	20  STORE("lotteryeveryXdeposit", 2)   // lottery will reward every X deposits
        // How much will lottery giveback in 1/10000 parts, granularity .01 %
	30  STORE("lotterygiveback", 9900)   // lottery will give reward 99% of deposits, 1 % is accumulated for owner to withdraw
	33  STORE("deposit_count", 0)   //  initial players
	34  STORE("deposit_total", 0)   //  total deposit of all players
	// 35 printf "Initialize executed"
	40 RETURN 0 
	End Function 
	
	
	
        // Function to tune lottery parameters
	Function TuneLotteryParameters(input Uint64, lotteryeveryXdeposit Uint64, lotterygiveback Uint64) Uint64
	10  dim key,stored_owner as String
	20  dim value_uint64 as Uint64
	30  IF LOAD("owner") == SIGNER() THEN GOTO 100  // check whether owner is real owner
	40  RETURN 1
	
	100  STORE("lotteryeveryXdeposit", lotteryeveryXdeposit)   // lottery will reward every X deposits
	130  STORE("lotterygiveback", value_uint64)   // how much will lottery giveback in 1/10000 parts, granularity .01 %
	140  RETURN 0 // return success
	End Function
	

	
	// This function is used to change owner 
	// owner is an string form of address 
	Function TransferOwnership(newowner String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  STORE("tmpowner",ADDRESS_RAW(newowner))
	40  RETURN 0
	End Function
	
	// Until the new owner claims ownership, existing owner remains owner
        Function ClaimOwnership() Uint64 
	10  IF LOAD("tmpowner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  STORE("owner",SIGNER()) // ownership claim successful
	40  RETURN 0
	End Function
	
	// If signer is owner, withdraw any requested funds
	// If everthing is okay, they will be showing in signers wallet
        Function Withdraw( amount Uint64) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  SEND_DERO_TO_ADDRESS(SIGNER(),amount)
	40  RETURN 0
	End Function
	
	// If signer is owner, provide him rights to update code anytime
        // make sure update is always available to SC
        Function UpdateCode( code String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  UPDATE_SC_CODE(code)
	40  RETURN 0
	End Function
	`

// run the test
func Test_Simulator_execution(t *testing.T) {
	s := SimulatorInitialize(nil)
	var addr *rpc.Address
	var err error

	if addr, err = rpc.NewAddress(strings.TrimSpace("deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p")); err != nil {
		panic(err)
	}

	var zerohash crypto.Hash

	s.AccountAddBalance(*addr, zerohash, 500)
	scid, gascompute, gasstorage, err := s.SCInstall(sc, map[crypto.Hash]uint64{}, rpc.Arguments{}, addr, 0)

	if err != nil {
		t.Fatalf("cannot install contract %s\n", err)
	}
	_ = gascompute
	_ = gasstorage

	// trigger first time lottery play
	gascompute, gasstorage, err = s.RunSC(map[crypto.Hash]uint64{zerohash: 45}, rpc.Arguments{{rpc.SCACTION, rpc.DataUint64, uint64(rpc.SC_CALL)}, {rpc.SCID, rpc.DataHash, scid}, rpc.Argument{"entrypoint", rpc.DataString, "Lottery"}}, addr, 0)
	if err != nil {
		t.Fatalf("cannot run contract %s\n", err)
	}

	w_sc_data_tree := Wrapped_tree(s.cache, s.ss, scid)
	stored_value, _ := LoadSCAssetValue(w_sc_data_tree, scid, zerohash)

	if 45 != stored_value {
		t.Fatalf("storage corruption dero value")
	}

	if uint64(45) != ReadSCValue(w_sc_data_tree, scid, "deposit_total") {
		t.Fatalf("storage corruption")
	}

	// trigger second time lottery play
	gascompute, gasstorage, err = s.RunSC(map[crypto.Hash]uint64{zerohash: 55}, rpc.Arguments{{rpc.SCACTION, rpc.DataUint64, uint64(rpc.SC_CALL)}, {rpc.SCID, rpc.DataHash, scid}, rpc.Argument{"entrypoint", rpc.DataString, "Lottery"}}, addr, 0)
	if err != nil {
		t.Fatalf("cannot run contract %s\n", err)
	}

	w_sc_data_tree = Wrapped_tree(s.cache, s.ss, scid)

	// only 1 is left ( which is profit)
	if stored_value, _ = LoadSCAssetValue(w_sc_data_tree, scid, zerohash); 1 != stored_value {
		t.Fatalf("storage corruption dero value")
	}

	// total deposit must be zero
	if uint64(0) != ReadSCValue(w_sc_data_tree, scid, "deposit_total") {
		t.Fatalf("storage corruption")
	}

}
