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
//import cryptorand "crypto/rand"

//import "encoding/binary"
import "encoding/hex"

//import "encoding/json"

import "github.com/romana/rlog"

//import "github.com/vmihailenco/msgpack"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derohe/crypto/ringct"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/rpc"

//import "github.com/deroproject/derohe/ddn"

//import "github.com/deroproject/derohe/structures"
//import "github.com/deroproject/derohe/blockchain/inputmaturity"
import "github.com/deroproject/derohe/cryptography/bn256"

/*
func (w *Wallet_Memory) Transfer_Simplified(addr string, value uint64, data []byte, scdata rpc.Arguments) (tx *transaction.Transaction, err error) {
	if sender, err := rpc.NewAddress(addr); err == nil {
		burn_value := uint64(0)
		return w.TransferPayload0(*sender, value, burn_value, 0, 0, false, data, scdata, false)
	}
	return
}
*/

// we should reply to an entry

// send amount to specific addresses
func (w *Wallet_Memory) TransferPayload0(transfers []rpc.Transfer, transfer_all bool, scdata rpc.Arguments, dry_run bool) (tx *transaction.Transaction, err error) {

	//    var  transfer_details structures.Outgoing_Transfer_Details
	w.transfer_mutex.Lock()
	defer w.transfer_mutex.Unlock()
	ringsize := uint64(w.account.Ringsize) // use wallet mixin, if mixin not provided

	bits_needed := make([]int, ringsize, ringsize)

	// if wallet is online,take the fees from the network itself
	// otherwise use whatever user has provided
	//if w.GetMode()  {
	fees_per_kb := w.dynamic_fees_per_kb // TODO disabled as protection while lots more testing is going on
	//rlog.Infof("Fees per KB %d\n", fees_per_kb)
	//}

	if fees_per_kb == 0 {
		fees_per_kb = config.FEE_PER_KB
	}

	for t := range transfers {
		var data []byte
		if data, err = transfers[t].Payload_RPC.CheckPack(transaction.PAYLOAD0_LIMIT); err != nil {
			return
		}

		if len(data) != transaction.PAYLOAD0_LIMIT {
			err = fmt.Errorf("Expecting exactly %d bytes data  but have  %d bytes", transaction.PAYLOAD0_LIMIT, len(data))
			return
		}
	}

	fees := uint64(0) //uint64(ringsize + 1) // start with zero fees
	//	expected_fee := uint64(0)

	if transfer_all {
		err = fmt.Errorf("Transfer all not supported")
		return
		transfers[0].Amount = w.account.Balance_Mature - fees
	}

	total_amount_required := uint64(0)

	for i := range transfers {
		total_amount_required += transfers[i].Amount + transfers[i].Burn
	}

	if total_amount_required > w.account.Balance_Mature {
		err = fmt.Errorf("Insufficent funds.")
		return
	}

	for t := range transfers {
		saddress := transfers[t].Destination

		if saddress == "" { // user skipped destination
			if transfers[t].SCID.IsZero() {
				err = fmt.Errorf("Main Destination cannot be empty")
				return
			}

			// we will try 5 times, to get a random ring ring member other than us, if ok, we move ahead
			for i := 0; i < 5; i++ {
				for _, k := range w.random_ring_members(transfers[t].SCID) {

					//fmt.Printf("%d ring %d '%s'\n",i,j,k)
					if k != w.GetAddress().String() {
						saddress = k
						transfers[t].Destination = k
						i = 1000 // break outer loop also
						break
					}
				}
			}
		}

		if saddress == "" {
			err = fmt.Errorf("could not obtain random ring member for scid %s", transfers[t].SCID)
			return
		}
		if _, err = rpc.NewAddress(saddress); err != nil {
			fmt.Printf("err processing address '%s' err '%s'\n", saddress, err)
			return
		}
	}

	emap := map[string]map[string][]byte{} //initialize all maps
	for i := range transfers {
		if _, ok := emap[string(transfers[i].SCID.String())]; !ok {
			emap[string(transfers[i].SCID.String())] = map[string][]byte{}
		}
	}

	var rings [][]*bn256.G1
	var max_bits_array []int

	_, self_e, _ := w.GetEncryptedBalanceAtTopoHeight(transfers[0].SCID, -1, w.GetAddress().String())
	if err != nil {
		fmt.Printf("self unregistered err %s\n", err)
		return
	}

	// WaitNewHeightBlock() // wait till a new block at new height is found
	// due to this we weill dispatch a new tx immediate after a block is found for better propagation

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

	for t := range transfers {

		var ring []*bn256.G1

		if transfers[t].SCID.IsZero() {
			ringsize = uint64(w.account.Ringsize)
		} else {
			ringsize = 2 // only for easier testing
		}

		bits_needed[0], self_e, err = w.GetEncryptedBalanceAtTopoHeight(transfers[t].SCID, -1, w.GetAddress().String())
		if err != nil {
			fmt.Printf("self unregistered err %s\n", err)
			return
		} else {
			emap[string(transfers[t].SCID.String())][w.account.Keys.Public.G1().String()] = self_e.Serialize()
			ring = append(ring, w.account.Keys.Public.G1())
		}

		var addr *rpc.Address
		if addr, err = rpc.NewAddress(transfers[t].Destination); err != nil {
			return
		}
		var dest_e *crypto.ElGamal
		bits_needed[1], dest_e, err = w.GetEncryptedBalanceAtTopoHeight(transfers[t].SCID, -1, addr.String())
		if err != nil {
			fmt.Printf(" t %d unregistered1 '%s' %s\n", t, addr, err)
			return
		} else {
			emap[string(transfers[t].SCID.String())][addr.PublicKey.G1().String()] = dest_e.Serialize()
			ring = append(ring, addr.PublicKey.G1())
		}

		ring_members_keys := make([]*bn256.G1, 0)
		ring_members_ebalance := make([]*crypto.ElGamal, 0)
		/*if len(w.account.RingMembers) < int(ringsize) {
			err = fmt.Errorf("We do not have enough ring members, expecting alteast %d but have only %d", int(ringsize), len(w.account.RingMembers))
			return
		}*/

		receiver_without_payment_id := addr.BaseAddress()
		for i, k := range w.random_ring_members(transfers[t].SCID) {
			if len(ring_members_keys)+2 < int(ringsize) && k != receiver_without_payment_id.String() && k != w.GetAddress().String() {

				//  fmt.Printf("%s     receiver %s   sender %s\n", k, receiver_without_payment_id.String(), w.GetAddress().String())
				var ebal *crypto.ElGamal
				var addr *rpc.Address
				bits_needed[i+2], ebal, err = w.GetEncryptedBalanceAtTopoHeight(transfers[t].SCID, -1, k)
				if err != nil {
					fmt.Printf(" unregistered %s\n", k)
					return
				}
				addr, err = rpc.NewAddress(k)
				if err != nil {
					return
				}

				emap[string(transfers[t].SCID.String())][addr.PublicKey.G1().String()] = ebal.Serialize()

				ring = append(ring, addr.PublicKey.G1())

				ring_members_keys = append(ring_members_keys, addr.PublicKey.G1())
				ring_members_ebalance = append(ring_members_ebalance, ebal)

				if len(ring_members_keys)+2 == int(ringsize) {
					break
				}

			}

		}

		rings = append(rings, ring)

		max_bits := 0
		for i := range bits_needed {
			if max_bits < bits_needed[i] {
				max_bits = bits_needed[i]
			}
		}
		max_bits_array = append(max_bits_array, max_bits)
	}

	if !dry_run {
		rlog.Debugf("we should build a TX now")
		tx = w.BuildTransaction(transfers, emap, rings, height, scdata, treehash_raw, max_bits_array)
	}

	return
}
