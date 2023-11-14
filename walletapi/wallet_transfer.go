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

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"
	"strings"
	
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

//import "sort"
//import "math/rand"
//import cryptorand "crypto/rand"

//import "encoding/binary"

//import "encoding/json"

//import "github.com/vmihailenco/msgpack"

//import "github.com/deroproject/derohe/crypto/ringct"

//import "github.com/deroproject/derohe/globals"

//import "github.com/deroproject/derohe/ddn"

//import "github.com/deroproject/derohe/structures"
//import "github.com/deroproject/derohe/blockchain/inputmaturity"

/*
func (w *Wallet_Memory) Transfer_Simplified(addr string, value uint64, data []byte, scdata rpc.Arguments) (tx *transaction.Transaction, err error) {
	if sender, err := rpc.NewAddress(addr); err == nil {
		burn_value := uint64(0)
		return w.TransferPayload0(*sender, value, burn_value, 0, 0, false, data, scdata, false)
	}
	return
}
*/

// This function set the address asset requested naively
// This can be a security flaw for exchanges or other services who accept integrated address without necessary checks
func (w *Wallet_Memory) TransferAssetFromAddress(transfers []rpc.Transfer, ringsize uint64, transfer_all bool, scdata rpc.Arguments, gasstorage uint64, dry_run bool) (tx *transaction.Transaction, err error) {
	// Update all asset used in transfer to use the one from integrated address if present
	for i := range transfers {
		transfer := transfers[i]
		// parse address
		var addr *rpc.Address
		if addr, err = rpc.NewAddress(transfer.Destination); err != nil {
			return nil, err

		}

		// if address contains RPC Asset, set it as SCID
		if addr.Arguments.Has(rpc.RPC_ASSET, rpc.DataHash) {
			scid := addr.Arguments.Value(rpc.RPC_ASSET, rpc.DataHash).(crypto.Hash)
			transfer.SCID = scid
		}
	}

	return w.TransferPayload0(transfers, ringsize, transfer_all, scdata, gasstorage, dry_run)
}

// we should reply to an entry

// send amount to specific addresses
func (w *Wallet_Memory) TransferPayload0(transfers []rpc.Transfer, ringsize uint64, transfer_all bool, scdata rpc.Arguments, gasstorage uint64, dry_run bool) (tx *transaction.Transaction, err error) {
	//    var  transfer_details structures.Outgoing_Transfer_Details
	w.transfer_mutex.Lock()
	defer w.transfer_mutex.Unlock()

	//if len(transfers) == 0 {
	//	return nil,  fmt.Error("transfers is nil, cannot send.")
	//}

	if ringsize == 0 {
		ringsize = uint64(w.account.Ringsize) // use wallet ringsize, if ringsize not provided
	} else { // we need to use supplied ringsize
		if ringsize&(ringsize-1) != 0 {
			err = fmt.Errorf("ringsize should be power of 2. value %d", ringsize)
			return
		}
		if !(ringsize >= config.MIN_RINGSIZE && ringsize <= config.MAX_RINGSIZE) {
			err = fmt.Errorf("ringsize out of range value %d", ringsize)
			return
		}
	}

	//ringsize = 2

	// if wallet is online,take the fees from the network itself
	// otherwise use whatever user has provided
	//if w.GetMode()  {
	fees_per_kb := w.dynamic_fees_per_kb // TODO disabled as protection while lots more testing is going on
	//rlog.Infof("Fees per KB %d\n", fees_per_kb)
	//}

	if fees_per_kb == 0 {
		fees_per_kb = config.FEE_PER_KB
	}

	// user wants to do an SC call, but doesn't want any transfer, so we will transfer 0 to a random account
	if len(scdata) >= 1 && len(transfers) == 0 {
		var zeroscid crypto.Hash
		for _, k := range w.Random_ring_members(zeroscid) {
			if k != w.GetAddress().String() { /// make sure random member is not equal to ourself
				transfers = append(transfers, rpc.Transfer{Destination: k, Amount: 0})
				logger.V(3).Info("Doing 0 transfer to", "random_address", k)
				break
			}
		}
	}

	if len(transfers) >= 1 {
		has_base := false
		for i := range transfers {
			if transfers[i].SCID.IsZero() {
				has_base = true
			}
		}

		// if we do not have base we can not detect fees. this restriction should be lifted at suitable time
		if !has_base {
			var zeroscid crypto.Hash
			for _, k := range w.Random_ring_members(zeroscid) {
				if k != w.GetAddress().String() { /// make sure random member is not equal to ourself
					transfers = append(transfers, rpc.Transfer{Destination: k, Amount: 0})
					logger.V(3).Info("Doing 0 transfer to", "random_address", k)
					break
				}
			}
		}

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

	//fees := (ringsize + 1) * fees_per_kb // start with zero fees
	//	expected_fee := uint64(0)

	if transfer_all {
		err = fmt.Errorf("Transfer all not supported")
		return
		//transfers[0].Amount = w.account.Balance_Mature - fees
	}

	total_amount_required := map[crypto.Hash]uint64{}
	for i := range transfers {
		total_amount_required[transfers[i].SCID] = total_amount_required[transfers[i].SCID] + transfers[i].Amount + transfers[i].Burn
	}

	var current_balance uint64
	for i := range transfers {
		current_balance, _, err = w.GetDecryptedBalanceAtTopoHeight(transfers[i].SCID, -1, w.GetAddress().String())

		if err != nil {
			return
		}
		if total_amount_required[transfers[i].SCID] > current_balance {
			err = fmt.Errorf("Insufficent funds for scid %s Need %s Actual %s", transfers[i].SCID, FormatMoney(total_amount_required[transfers[i].SCID]), FormatMoney(current_balance))
			return
		}
	}

	for t := range transfers {

		if transfers[t].Destination == "" { // user skipped destination
			if transfers[t].SCID.IsZero() {
				err = fmt.Errorf("Main Destination cannot be empty")
				return
			}

			// we will try x times, to get a random ring ring member other than us, if ok, we move ahead
			ring_count := 0
			for i := 0; i < 20; i++ {
				//fmt.Printf("getting random ring member %d\n", i)
				scid := transfers[t].SCID
				if i > 17 { // if we cannot obtain ring member is 17 tries, choose  ring member from zero
					var zeroscid crypto.Hash
					scid = zeroscid
				}
				for _, k := range w.Random_ring_members(scid) {
					if k != w.GetAddress().String() {
						transfers[t].Destination = k
						i = 1000000 // break outer loop also
						ring_count++
						break
					}
				}
			}

		}

		if transfers[t].Destination == "" {
			err = fmt.Errorf("could not obtain random ring member for scid %s", transfers[t].SCID)
			return
		}

		// try to resolve name to address here
		if _, err = rpc.NewAddress(transfers[t].Destination); err != nil {
			if transfers[t].Destination, err = w.NameToAddress(transfers[t].Destination); err != nil {
				err = fmt.Errorf("could not decode name or address err '%s' name '%s'\n", err, transfers[t].Destination)
				return
			}
		}
	}

	//fmt.Printf("transfers %+v\n", transfers)

	var rings [][]*bn256.G1
	var rings_balances [][][]byte //initialize all maps

	var max_bits_array []int

	topoheight := int64(-1)
	var block_hash crypto.Hash

	var zeroscid crypto.Hash

	// noncetopo should be verified for all ring members simultaneously
	// this can lead to tx rejection
	// we currently bypass this since random members are chosen which have not been used in last 5 block
	_, noncetopo, block_hash, self_e, err := w.GetEncryptedBalanceAtTopoHeight(zeroscid, -1, w.GetAddress().String())
	if err != nil {
		err = fmt.Errorf("could not obtain encrypted balance for self err %s\n", err)
		return
	}

	// TODO, we should check nonce for base token and other tokens at the same time
	// right now, we are probably using a bit of luck here
	if daemon_topoheight >= int64(noncetopo)+3 { // if wallet has not been recently used, increase probability  of user's tx being successfully mined

		topoheight = daemon_topoheight - 3
	}

	_, _, block_hash, self_e, _ = w.GetEncryptedBalanceAtTopoHeight(transfers[0].SCID, topoheight, w.GetAddress().String())
	if err != nil {
		return
	}

	er, err := w.GetSelfEncryptedBalanceAtTopoHeight(transfers[0].SCID, topoheight)
	if err != nil {
		err = fmt.Errorf("could not obtain encrypted balance for self err %s\n", err)
		return
	}
	height := uint64(er.Height)
	block_hash = er.BlockHash
	topoheight = er.Topoheight
	treehash := er.Merkle_Balance_TreeHash

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
		var ring_balances [][]byte

		/*	if transfers[t].SCID.IsZero() {
				ringsize = uint64(ringsize)
			} else {
				ringsize = ringsize // only for easier testing
			}
		*/

		bits_needed := make([]int, ringsize, ringsize)

		bits_needed[0], _, _, self_e, err = w.GetEncryptedBalanceAtTopoHeight(transfers[t].SCID, topoheight, w.GetAddress().String())
		if err != nil {
			fmt.Printf("self unregistered err %s\n", err)
			return
		} else {
			ring_balances = append(ring_balances, self_e.Serialize())
			ring = append(ring, w.account.Keys.Public.G1())
		}

		var addr *rpc.Address
		if addr, err = rpc.NewAddress(transfers[t].Destination); err != nil {
			return
		}

		if addr.IsIntegratedAddress() && addr.Arguments.Validate_Arguments() != nil {
			err = fmt.Errorf("Integrated Address  arguments could not be validated.")
			return
		}

		if addr.IsIntegratedAddress() && len(transfers[t].Payload_RPC) == 0 {
			for _, arg := range addr.Arguments {
				if arg.Name == rpc.RPC_DESTINATION_PORT && addr.Arguments.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) {
					transfers[t].Payload_RPC = append(transfers[t].Payload_RPC, rpc.Argument{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: addr.Arguments.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64)})
					continue
				} else {
					// Shouldn't we jus replicate them in payload_rpc ?
					err = fmt.Errorf("integrated address used, but don't know how to process %+v", addr.Arguments)
					return
				}
			}
		}

		var dest_e *crypto.ElGamal
		bits_needed[1], _, _, dest_e, err = w.GetEncryptedBalanceAtTopoHeight(transfers[t].SCID, topoheight, addr.BaseAddress().String())
		if err != nil {
			fmt.Printf(" t %d unregistered1 '%s' %s\n", t, addr, err)
			return
		} else {
			ring_balances = append(ring_balances, dest_e.Serialize())
			ring = append(ring, addr.PublicKey.G1())
		}

		/*if len(w.account.RingMembers) < int(ringsize) {
			err = fmt.Errorf("We do not have enough ring members, expecting alteast %d but have only %d", int(ringsize), len(w.account.RingMembers))
			return
		}*/

		receiver_without_payment_id := addr.BaseAddress()

		//sending to self is not supported
		if w.GetAddress().String() == receiver_without_payment_id.String() {
			err = fmt.Errorf("Sending to self is not supported")
			return
		}

		deduplicator := map[string]bool{}
		deduplicator[receiver_without_payment_id.String()] = true
		deduplicator[w.GetAddress().String()] = true

		for ringsize != 2 {
			probable_members := w.Random_ring_members(transfers[t].SCID)
			if len(probable_members) <= 40 { // we do not have enough ring members for sure, extract ring members from base
				var zeroscid crypto.Hash
				probable_members = w.Random_ring_members(zeroscid)
			}
			for _, k := range probable_members {
				if _, collision := deduplicator[k]; collision {
					continue
				}
				deduplicator[k] = true
				if len(ring_balances) < int(ringsize) && k != receiver_without_payment_id.String() && k != w.GetAddress().String() {
					var addr_member *rpc.Address
					//fmt.Printf("t:%d len %d %s     receiver %s   sender %s\n",t,len(ring_balances),  k, receiver_without_payment_id.String(), w.GetAddress().String())
					var ebal *crypto.ElGamal

					bits_needed[len(ring_balances)], _, _, ebal, err = w.GetEncryptedBalanceAtTopoHeight(transfers[t].SCID, -1, k)
					if err != nil {
						fmt.Printf(" unregistered %s\n", k)
						return
					}
					if addr_member, err = rpc.NewAddress(k); err != nil {
						return
					}

					ring_balances = append(ring_balances, ebal.Serialize())
					ring = append(ring, addr_member.PublicKey.G1())

					if len(ring_balances) == int(ringsize) {
						goto ring_members_collected
					}

				}
			}

		}
	ring_members_collected:

		rings = append(rings, ring)
		rings_balances = append(rings_balances, ring_balances)

		max_bits := 0
		for i := range bits_needed {
			if max_bits < bits_needed[i] {
				max_bits = bits_needed[i]
			}
		}
		max_bits_array = append(max_bits_array, max_bits)
	}
	max_bits := 0
	for i := range max_bits_array {
		if max_bits < max_bits_array[i] {
			max_bits = max_bits_array[i]
		}
	}
	max_bits += 6 // extra 6 bits


	//If this is a view only wallet, prepare and send the transaction to the offline wallet for signing.
	//Import the signed transaction again and continue with the normal work flow to submit the transaction
	//to the network
	if (w.ViewOnly()==true) {
		// Calculate an estimate of the full transaction cost, to see if the balance has enough funds:
		var spend uint64
		var fees uint64		
		spend=0
		fees=0
		fees_done:=false
		
		len_publickeylist:=len(rings[0])
		
		for t := range transfers {
			spend = spend + transfers[t].Amount + transfers[t].Burn
			
			//From transaction builder, use the estimate fees calculator:
			var asset transaction.AssetPayload

			asset.SCID = transfers[t].SCID
			asset.BurnValue = transfers[t].Burn

			if fees == 0 && asset.SCID.IsZero() && !fees_done {
		             fees = fees + uint64(len(transfers)+2)*uint64((float64(config.FEE_PER_KB)*float64(float32(len_publickeylist/16)+w.GetFeeMultiplier())))
		             if data, err := scdata.MarshalBinary(); err != nil {
                		     panic(err)
		             } else {
                		     fees = fees + (uint64(len(data))*15)/10
		             }
		             fees_done = true
			}
		}
		//Fee is applied for each transfer in the transaction:
		fees = fees * uint64(len(transfers))
		
//		fmt.Printf("spends: %d\n", spend);
		if (current_balance < (spend+fees)) {
			err = fmt.Errorf("Spend amount (%d) + estimated fees (%d) is more than your current balance: %d\n",spend,fees,current_balance)
			return
		}
	
		//Parameter   [0]: Project - Dero='dero'
		//            [1]: Version - Layout of the command fields
		//            [2]: Command: sign_offline
		// Version 1: [3] Array of transfers (outputs)
		//            [4] Array of ring balances
		//            [5] Array of rings  
		//            [6] block_hash
		//            [7] height
		//            [8] Array of scdata
		//            [9] treehash
		//            [10] max_bits
		//            [11] gasstorage
		//            [12] Checksum of all the characters in the command.
		var sReturn string 
		var sChecksumInput string
	  
		sReturn="dero 1 sign_offline ";
		sChecksumInput="dero 1 sign_offline ";
	  
		var counter=0
		
//		fmt.Print("transfers: %d\n",len(transfers))
		sChecksumInput+="transfers:";  
		sReturn=sReturn+"'[";  
		counter=0
		for t := range transfers {
			baRPC, err := transfers[t].Payload_RPC.MarshalBinary()
			if err!=nil {
				panic("Could not convert payload_rpc to binary")
			}
			hexRPC := hex.EncodeToString(baRPC)
			sTmp:=fmt.Sprintf("{\"SCID\":\"%s\",\"Destination\":\"%s\",\"Amount\":%d,\"Burn\":%d,\"RPC\":\"%s\"}", transfers[t].SCID, transfers[t].Destination, transfers[t].Amount, transfers[t].Burn, hexRPC )
			sReturn=sReturn+sTmp;
			 	    
			sTmp=fmt.Sprintf("\"%s\" \"%s\" %d %d \"%s\"", transfers[t].SCID, transfers[t].Destination, transfers[t].Amount, transfers[t].Burn, hexRPC )
			sChecksumInput+=" "+sTmp		  
	    
			//Last entry: Must close the array with ]'
			//otherwide start the next entry with a ,
			if (counter == len(transfers)-1) {
			      sReturn=sReturn+"]' ";
			} else {
			      sReturn=sReturn+",";
			}    
			counter++
		}
			
//		fmt.Printf("rings_balances: %d\n",len(rings_balances))
		var counter1=0
		var counter2=0
		
		sChecksumInput+=" rings_balances:"
		sReturn=sReturn+"'["
		for t := range rings_balances {		
		    //fmt.Printf("rings_balances{%d}:{%d} entries\n",counter1,len(rings_balances[t]))
			sReturn=sReturn+"{";
			counter2=0
			for u := range rings_balances[t] {
				sTmp := fmt.Sprintf("%x",rings_balances[t][u])
				sReturn = sReturn + sTmp
				sChecksumInput+=" "+sTmp
		
				//Last entry: Must close the array with ]'
				//otherwide start the next entry with a ,
				if (counter2 == len(rings_balances[t])-1) {
					sReturn=sReturn+"}";
				} else {
					sReturn=sReturn+",";
				}
				counter2++
			}
		    
			//Last entry: Must close the array with ]'
			//otherwide start the next entry with a ,
			if (counter1 == len(rings_balances)-1) {
				sReturn=sReturn+"]' ";
			} else {
				sReturn=sReturn+",";
			}	    
			counter1++
		}	
	  
//		fmt.Printf("rings:{%d} entries\n", len(rings))
		sChecksumInput+=" rings:";  
		sReturn=sReturn+"'[";
	  
		counter1=0
		counter2=0
		for t := range rings {
			//fmt.Printf("ring{%d}:{%d} entries\n",counter1,len(rings[t]))
			sReturn=sReturn+"{";
			counter2=0
			for u := range rings[t] {
				sTmp := fmt.Sprintf("%x",rings[t][u].Marshal() )
				sReturn = sReturn + sTmp
				sChecksumInput+=" "+sTmp
		
				if (counter2 == len(rings_balances[t])-1) {
					sReturn=sReturn+"}";
				} else {
					sReturn=sReturn+",";
				}      
				counter2++
			}
			
			if (counter1 == len(rings_balances)-1) {
				sReturn=sReturn+"]' ";
			} else {
				sReturn=sReturn+",";
			}			
			counter1++
		}
		
	  
		sTmp:=fmt.Sprintf("\"%s\" ",block_hash)
		sReturn=sReturn + sTmp
		sChecksumInput+=" "+sTmp  

		sTmp=fmt.Sprintf("%d ",height)
		sReturn=sReturn + sTmp
		sChecksumInput+=sTmp  

		sTmp=fmt.Sprintf("'%x' ",scdata)
		sReturn=sReturn + sTmp
		sChecksumInput+=sTmp  

		sTmp=fmt.Sprintf("\"%x\" ",treehash_raw)
		sReturn=sReturn + sTmp
		sChecksumInput+=sTmp  
	  
		sTmp=fmt.Sprintf("%d ",max_bits)
		sReturn=sReturn + sTmp
		sChecksumInput+=sTmp  
	  
		sTmp=fmt.Sprintf("%x",gasstorage)
		sReturn=sReturn + sTmp
		sChecksumInput+=sTmp  
	  
	  
		//Parameter [12]: checksum
		//A simple checksum of the full string, to detect copy/paste errors between the wallets
		//The checksum equals the sum of the ASCII values of all the characters in the string:   
		iChecksum:=0x01;
		for t := range sChecksumInput {
			iVal := int(sChecksumInput[t])
			iChecksum = iChecksum + iVal;
		}
		sTmp = fmt.Sprintf(";%d",iChecksum)
		sReturn=sReturn + sTmp  
		
	        //--------------------------------------------------------------------------------------
		// Compiling transaction data completed. Now need to exchange it with the offline wallet
		// to get it signed
		  
		_ = os.Remove("./transaction")
	  
		if _, err = os.Stat("./transaction"); err == nil {
		    err = fmt.Errorf("Could not delete ./transaction")
		    return
		}
		
		//The signed file is created by the offline wallet. We may not have permissions to erase the
		// file. Let the user perform it manually
		_ = os.Remove("./signed")
		if _, err = os.Stat("./signed"); err == nil {
		    err = fmt.Errorf("Found old signed transaction at ./signed. Delete the file before starting a new transaction.\n")
		    return
		}
	  
		fmt.Printf("Saved transaction data to: ./transaction. Transfer it to the offline (signing) wallet and have it signed there.\n" )
	  
		baData := []byte(sReturn)
		err = os.WriteFile("./transaction", baData, 0644)
		if err!=nil {
		    err = fmt.Errorf("Error saving file. %s\n",err)
		    return
		}

		bFound:=0
		counter=0
		//TODO: what is the longest we can wait to submit a transaction and still have it processed by the network?		
		fmt.Printf("Waiting 30 seconds for the signed transaction from the offline wallet at ./signed\n");
		for counter <= 30 {  
			if _, err := os.Stat("./signed"); err == nil {
				bFound=1
				break;
			}
			counter++
			time.Sleep(1 * time.Second)
		}
	  
		if (bFound==0) {
		    err = fmt.Errorf("Could not find the signed transaction\n");
		    return;
		}
	  
		baData, err = os.ReadFile("./signed")
		if err!=nil {
			err = fmt.Errorf("Could not read from ./signed. Check the file permissions.\n");
			return;
		}		
		fmt.Printf("Read %d bytes from ./signed. ",len(baData))

		tx=nil
                //Parameter   [0]: Project - 'dero'
                //            [1]: Version - Layout of the command fields
                //            [2]: Command: signed
                // Version 1: [3] Signed transaction
                //            [4] Checksum of all the characters in the data stream
                sInput := string(baData[:])
                saParts := strings.Split(sInput,";")
                                
                if (len(saParts) != 2) {
                	fmt.Printf("Invalid number of parts in the transaction. Expected 2, found %d\n", len(saParts))
                	return
		}
                                
                sProtocolChecksum := saParts[1]
                iCalculatedChecksum:=0x01;
                for t := range saParts[0] {
			iVal := int(saParts[0][t])
			iCalculatedChecksum = iCalculatedChecksum + iVal;
		}
		sCalculatedChecksum := fmt.Sprintf("%d",iCalculatedChecksum)

		if (sProtocolChecksum!=sCalculatedChecksum) {
			fmt.Printf("The checksum of the request data is invalid. Protocol: '%s', Calculates: '%s'\n", sProtocolChecksum, sCalculatedChecksum)
			return
		}
 
 		saFields := strings.Split(saParts[0]," ")               
                if (len(saFields) != 4) {
                	err = fmt.Errorf("Invalid number of parts in the transaction. Expected 4, found %d\n", len(saFields))
			return;
                }

		if  (saFields[0] != "dero") {
			err = fmt.Errorf("Expected Dero communication, Found %s\n",saFields[1]);
                        return;
		}

                if (saFields[1] != "1") {
                	err = fmt.Errorf("Only transaction version 1 supported. Found %s\n",saFields[2])
                        return
		}
		
                if (saFields[2] != "signed") {
                	err = fmt.Errorf("Transaction doesn't start with 'sign_offline'\n")
                        return
		}
		                                
		tx = new(transaction.Transaction)
		baData,err = hex.DecodeString( saFields[3] )
		if err!= nil {
			err = fmt.Errorf("Could not decode the signed transaction.\n")
			tx=nil
			return		
		}
		err = tx.Deserialize(baData)
		if err!=nil {
			err = fmt.Errorf("Could not deserialise the signed transaction.\n")
			tx=nil
			return
		}
		
		//Remove old files
		_ = os.Remove("./signed")
		
		fmt.Printf("The checksum passed. Continue to submit the transaction\n\n")	
		
		return;
	}
	

	if !dry_run {
		tx = w.BuildTransaction(transfers, rings_balances, rings, block_hash, height, scdata, treehash_raw, max_bits, gasstorage)
	}

	if tx == nil {
		err = fmt.Errorf("somehow the tx could not be built, please retry")
	}

	return
}
