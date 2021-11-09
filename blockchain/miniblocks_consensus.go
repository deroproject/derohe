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

package blockchain

import "fmt"

//import "time"
import "bytes"
import "encoding/binary"

//import "github.com/go-logr/logr"

//import "golang.org/x/xerrors"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/cryptography/crypto"

const miniblock_genesis_distance = 0
const miniblock_normal_distance = 2

// last miniblock must be extra checked for corruption/attacks
func (chain *Blockchain) Verify_MiniBlocks_HashCheck(cbl *block.Complete_Block) (err error) {
	last_mini_block := cbl.Bl.MiniBlocks[len(cbl.Bl.MiniBlocks)-1]

	if last_mini_block.Genesis && len(cbl.Bl.MiniBlocks) == 1 {
		return nil
	}

	txshash := cbl.Bl.GetTXSHash()
	block_header_hash := cbl.Bl.GetHashWithoutMiniBlocks()

	for i := range last_mini_block.Check {
		if last_mini_block.Check[i] != txshash[i]^block_header_hash[i] {
			return fmt.Errorf("MiniBlock has corrupted header.")
		}
	}
	return nil
}

// verifies the consensus rules completely for miniblocks
func (chain *Blockchain) Verify_MiniBlocks(bl block.Block) (err error) {

	if bl.Height == 0 && len(bl.MiniBlocks) != 0 {
		err = fmt.Errorf("Genesis block cannot have miniblocks")
		return
	}

	if bl.Height == 0 {
		return nil
	}

	if bl.Height != 0 && len(bl.MiniBlocks) == 0 {
		err = fmt.Errorf("All blocks except genesis must have miniblocks")
		return
	}

	for _, mbl := range bl.MiniBlocks {
		if mbl.Timestamp > uint64(globals.Time().UTC().UnixMilli())+50 { // 50 ms passing allowed
			//block_logger.Error(fmt.Errorf("MiniBlock has invalid timestamp from future"), "rejecting","current time",globals.Time().UTC(),"miniblock_time", mbl.GetTimestamp(),"i",i)
			return errormsg.ErrInvalidTimestamp
		}
	}

	// check whether the genesis blocks are all equal
	for _, mbl := range bl.MiniBlocks {
		if mbl.Genesis { // make sure all genesis blocks point to all the actual tips

			if bl.Height != binary.BigEndian.Uint64(mbl.Check[:]) {
				return fmt.Errorf("MiniBlock has invalid height")
			}
			if len(bl.Tips) != int(mbl.PastCount) {
				return fmt.Errorf("MiniBlock has wrong number of tips")
			}
			if len(bl.Tips) == 0 {
				panic("all miniblocks genesis must point to tip")
			} else if len(bl.Tips) == 1 {
				if !bytes.Equal(mbl.Check[8:8+12], bl.Tips[0][0:12]) {
					return fmt.Errorf("MiniBlock has invalid tip")
				}
			} else if len(bl.Tips) == 2 {
				if !(bytes.Equal(mbl.Check[8:8+12], bl.Tips[0][0:12]) || bytes.Equal(mbl.Check[8:8+12], bl.Tips[1][0:12])) {
					return fmt.Errorf("MiniBlock has invalid tip")
				}
				if !(bytes.Equal(mbl.Check[8+12:], bl.Tips[1][0:12]) || bytes.Equal(mbl.Check[8+12:], bl.Tips[1][0:12])) {
					return fmt.Errorf("MiniBlock has invalid second tip")
				}

				if bytes.Equal(mbl.Check[8:8+12], mbl.Check[8+12:]) {
					return fmt.Errorf("MiniBlock refers to same tip twice")
				}
			} else {
				panic("we only support  2 tips")
			}
		}
	}

	// we should draw the dag and make sure each and every one is connected
	{
		tmp_collection := block.CreateMiniBlockCollection()
		for _, tmbl := range bl.MiniBlocks {
			if err, ok := tmp_collection.InsertMiniBlock(tmbl); !ok {
				return err
			}
		}

		tips := tmp_collection.GetAllTips() // we should only receive a single tip
		if len(tips) != 1 {
			return fmt.Errorf("MiniBlock consensus should have only 1 tip")
		}

		if tips[0].GetHash() != bl.MiniBlocks[len(bl.MiniBlocks)-1].GetHash() {
			return fmt.Errorf("MiniBlock consensus last tip is placed wrong")
		}

		history := block.GetEntireMiniBlockHistory(tips[0])
		if len(history) != len(bl.MiniBlocks) {
			return fmt.Errorf("MiniBlock dag is not completely connected")
		}

		// check condition where tips cannot be referred to long into past
		for _, mbl := range history {
			if !mbl.Genesis {
				if mbl.PastCount == 2 {
					p1 := tmp_collection.Get(mbl.Past[0])
					p2 := tmp_collection.Get(mbl.Past[1])

					if p1.Distance == p2.Distance ||
						(p1.Distance > p2.Distance && (p1.Distance-p2.Distance) <= miniblock_genesis_distance && p2.Genesis) || // this will limit forking
						(p2.Distance > p1.Distance && (p2.Distance-p1.Distance) <= miniblock_genesis_distance && p1.Genesis) || // this will limit forking
						(p1.Distance > p2.Distance && (p1.Distance-p2.Distance) <= miniblock_normal_distance) || // give some freeway to miners
						(p2.Distance > p1.Distance && (p2.Distance-p1.Distance) <= miniblock_normal_distance) { // give some freeway to miners

					} else {
						return fmt.Errorf("MiniBlock dag is well formed, but tip referred is too long in distance")
					}

				}
			}
		}
	}

	return nil
}

// for the first time, we are allowing programmable consensus rules based on miniblocks
// this should be power of 2
// the below rule works as follows
//  say we need  blocktime of 15 sec
// so if we configure dynamism parameter to 7
// so some blocks will contain say 13 miniblocks, some will contain 20 miniblock

func (chain *Blockchain) Check_Dynamism(mbls []block.MiniBlock) (err error) {

	if chain.simulator { // simulator does not need dynamism check for simplicity
		return nil
	}

	dynamism := uint(7)

	if dynamism&(dynamism+1) != 0 {
		return fmt.Errorf("dynamic parameter must be a power of 2")
	}

	minimum_no_of_miniblocks := uint(config.BLOCK_TIME) - dynamism - 1

	if uint(len(mbls)) < minimum_no_of_miniblocks {
		return fmt.Errorf("more miniblocks required to complete a block required %d have %d", minimum_no_of_miniblocks, len(mbls))
	}

	last_mini_block := mbls[len(mbls)-1]
	last_mini_block_hash := last_mini_block.GetHash()
	if uint(last_mini_block_hash[31])&dynamism != dynamism {
		return fmt.Errorf("more miniblocks are required to complete a block.")
	}
	return nil
}

// insert a miniblock to chain and if successfull inserted, notify everyone in need
func (chain *Blockchain) InsertMiniBlock(mbl block.MiniBlock) (err error, result bool) {

	var miner_hash crypto.Hash
	copy(miner_hash[:], mbl.KeyHash[:])
	if !chain.IsAddressHashValid(true, miner_hash) {
		logger.V(1).Error(err, "Invalid miner address")
		err = fmt.Errorf("Invalid miner address")
		return err, false
	}

	if err, result = chain.MiniBlocks.InsertMiniBlock(mbl); result == true {
		chain.RPC_NotifyNewMiniBlock.L.Lock()
		chain.RPC_NotifyNewMiniBlock.Broadcast()
		chain.RPC_NotifyNewMiniBlock.L.Unlock()
	}
	return err, result
}
