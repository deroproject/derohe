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

// the objective of this file is to implememt a pool which sends and retries transactions until they are accepted by the chain

//import "fmt"

//import "encoding/binary"
//import "encoding/hex"

//import "encoding/json"

//import "github.com/romana/rlog"

//import "github.com/vmihailenco/msgpack"

//import "github.com/deroproject/derohe/config"
//import "github.com/deroproject/derohe/crypto"

//import "github.com/deroproject/derohe/crypto/ringct"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/address"

//import "github.com/deroproject/derohe/structures"
//import "github.com/deroproject/derohe/blockchain/inputmaturity"
//import "github.com/deroproject/derohe/crypto/bn256"

type Wallet_Pool []Wallet_Pool_Entry

type Wallet_Pool_Entry struct {
	Addr                []address.Address
	Amount              []uint64
	RingSize            uint64
	Transfer_Everything bool
	Trigger_Height      int64
}

// send amount to specific addresses
func (w *Wallet_Memory) PoolTransfer(addr []address.Address, amount []uint64, unlock_time uint64, payment_id_hex string, fees_per_kb uint64, ringsize uint64, transfer_all bool) (tx *transaction.Transaction, err error) {

	//    var  transfer_details structures.Outgoing_Transfer_Details

	var entry Wallet_Pool_Entry

	for i := range addr {
		entry.Addr = append(entry.Addr, addr[i])
		entry.Amount = append(entry.Amount, amount[i])

	}
	entry.RingSize = ringsize
	entry.Transfer_Everything = transfer_all

	w.account.Pool = append(w.account.Pool, entry)

	return
}
