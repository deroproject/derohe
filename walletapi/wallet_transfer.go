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

//import "sort"
//import "math/rand"
import cryptorand "crypto/rand"

//import "encoding/binary"
import "encoding/hex"

//import "encoding/json"

import "github.com/romana/rlog"

//import "github.com/vmihailenco/msgpack"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/crypto"

//import "github.com/deroproject/derohe/crypto/ringct"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/address"

//import "github.com/deroproject/derohe/structures"
//import "github.com/deroproject/derohe/blockchain/inputmaturity"
import "github.com/deroproject/derohe/crypto/bn256"

// send amount to specific addresses
func (w *Wallet_Memory) Transfer(addr []address.Address, amount []uint64, unlock_time uint64, payment_id_hex string, fees_per_kb uint64, ringsize uint64, transfer_all bool) (tx *transaction.Transaction, err error) {

	//    var  transfer_details structures.Outgoing_Transfer_Details
	w.transfer_mutex.Lock()
	defer w.transfer_mutex.Unlock()
	if ringsize == 0 {
		ringsize = uint64(w.account.Ringsize) // use wallet mixin, if mixin not provided
	}

	//ringsize = uint64(w.account.Ringsize)
	if ringsize < 2 { // enforce minimum mixin
		ringsize = 2
	}

	// if wallet is online,take the fees from the network itself
	// otherwise use whatever user has provided
	//if w.GetMode()  {
	fees_per_kb = w.dynamic_fees_per_kb // TODO disabled as protection while lots more testing is going on
	rlog.Infof("Fees per KB %d\n", fees_per_kb)
	//}

	if fees_per_kb == 0 {
		fees_per_kb = config.FEE_PER_KB
	}

	//var txw *TX_Wallet_Data
	if len(addr) != len(amount) {
		err = fmt.Errorf("Count of address and amounts mismatch")
		return
	}

	if len(addr) < 1 {
		err = fmt.Errorf("Destination address missing")
		return
	}

	var payment_id []byte // we later on find WHETHER to include it, encrypt it depending on length

	// if payment  ID is provided explicity, use it
	if payment_id_hex != "" {
		payment_id, err = hex.DecodeString(payment_id_hex) // payment_id in hex
		if err != nil {
			return
		}

		if len(payment_id) != 8 {
			err = fmt.Errorf("Payment ID must be  16 hex chars 8 byte")
			return
		}

	}

	// only only single payment id
	for i := range addr {
		if addr[i].IsIntegratedAddress() && payment_id_hex != "" {
			err = fmt.Errorf("Payment ID provided in both integrated address and separately")
			return
		}
	}

	// if integrated address payment id present , normal payment id must not be provided
	for i := range addr {
		if addr[i].IsIntegratedAddress() {
			if len(payment_id) > 0 { // a transaction can have only single encrypted payment ID
				err = fmt.Errorf("More than 1 integrated address provided")
				return
			}
			payment_id = addr[i].PaymentID
		}
	}

	if len(payment_id) == 0 { // we still have no payment id, give ourselves a random id
		payment_id = make([]byte, 8, 8)
		cryptorand.Read(payment_id[:])
		payment_id[0] = 'N'
		payment_id[1] = 'O'
	}

	fees := uint64(ringsize + 1) // start with zero fees
	//	expected_fee := uint64(0)
	total_amount_required := uint64(0)

	for i := range amount {
		if amount[i] == 0 { // cannot send 0  amount

		}
		total_amount_required += amount[i]
	}

	if transfer_all {
		amount[0] = w.account.Balance_Mature - fees
	}

	if total_amount_required > w.account.Balance_Mature {
		err = fmt.Errorf("Insufficent funds.")
		return
	}

	previous := w.account.Balance_Result.Data

	self_e, err := w.GetEncryptedBalance("", w.GetAddress().String())
	if err != nil {
		return
	}

	WaitNewHeightBlock() // wait till a new block at new height is found
	// due to this we weill dispatch a new tx immediate after a block is found for better propagation

	self_e, err = w.GetEncryptedBalance("", w.GetAddress().String())
	if err != nil {
		return
	}

	if w.account.Balance_Result.Data != previous { // wallet is stale, we need to update our balance
		w.DecodeEncryptedBalance() // try to decode balance
	}

	height := w.Daemon_Height
	treehash := w.Merkle_Balance_TreeHash

	treehash_raw, err := hex.DecodeString(treehash)
	if err != nil {
		return
	}
	if len(treehash_raw) != 32 {
		err = fmt.Errorf("roothash is not of 32 bytes, probably daemon corruption '%s'", treehash)
		return
	}

	dest_e := make([]*crypto.ElGamal, len(addr), len(addr))
	for i := range addr {
		dest_e[0], err = w.GetEncryptedBalance(treehash, addr[i].String())
		if err != nil {
			return
		}
	}

	ring_members_keys := make([]*bn256.G1, 0)
	ring_members_ebalance := make([]*crypto.ElGamal, 0)
	if len(w.account.RingMembers) < int(ringsize) {
		err = fmt.Errorf("We do not have enough ring members, expecting alteast %d but have only %d", int(ringsize), len(w.account.RingMembers))
		return
	}

	receiver_without_payment_id, _ := addr[0].Split()
	for k, _ := range w.account.RingMembers {

		if len(ring_members_keys)+2 < int(ringsize) && k != receiver_without_payment_id.String() && k != w.GetAddress().String() {

			//  fmt.Printf("%s     receiver %s   sender %s\n", k, receiver_without_payment_id.String(), w.GetAddress().String())
			var ebal *crypto.ElGamal
			var addr *address.Address
			ebal, err = w.GetEncryptedBalance(treehash, k)
			if err != nil {
				return
			}
			addr, err = address.NewAddress(k)
			if err != nil {
				return
			}

			ring_members_keys = append(ring_members_keys, addr.PublicKey.G1())
			ring_members_ebalance = append(ring_members_ebalance, ebal)

			if len(ring_members_keys)+2 == int(ringsize) {
				break
			}

		}

	}

	rlog.Debugf("we should build a TX now ring members %d:%d payment_id %x \n", len(ring_members_keys), len(ring_members_ebalance), payment_id)

	tx = BuildTransaction(w.account.Keys.Public.G1(), w.account.Keys.Secret.BigInt(), addr[0].PublicKey.G1(), self_e, dest_e[0], w.account.Balance_Mature, amount[0], ring_members_keys, ring_members_ebalance, fees, height, payment_id, treehash_raw)

	return
}
