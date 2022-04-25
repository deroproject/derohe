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

import (
	"fmt"
	"github.com/deroproject/derohe/rpc"
	"io/ioutil"
	"log"
	"os"
	"time"
)

//import "time"

import "encoding/binary"

import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"

import "golang.org/x/crypto/sha3"

// last miniblock must be extra checked for corruption/attacks
func (chain *Blockchain) Verify_MiniBlocks_HashCheck(cbl *block.Complete_Block) (err error) {
	last_mini_block := cbl.Bl.MiniBlocks[len(cbl.Bl.MiniBlocks)-1]

	if !last_mini_block.HighDiff {
		return fmt.Errorf("corrupted block")
	}

	if !last_mini_block.Final {
		return fmt.Errorf("corrupted block")
	}

	block_header_hash := sha3.Sum256(cbl.Bl.SerializeWithoutLastMiniBlock())
	for i := 0; i < 16; i++ {
		if last_mini_block.KeyHash[i] != block_header_hash[i] {
			return fmt.Errorf("MiniBlock has corrupted header expected %x  actual %x", block_header_hash[:], last_mini_block.KeyHash[:])
		}
	}
	return nil
}

// verifies the consensus rules completely for miniblocks
func Verify_MiniBlocks(bl block.Block) (err error) {

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

	final_count := 0
	for _, mbl := range bl.MiniBlocks {
		if mbl.Final { // 50 ms passing allowed
			final_count++
		}
	}
	if final_count < 1 {
		err = fmt.Errorf("No final miniblock")
		return
	}

	if uint64(len(bl.MiniBlocks)) != (config.BLOCK_TIME - config.MINIBLOCK_HIGHDIFF + 1) {
		err = fmt.Errorf("incorrect number of miniblocks expected %d actual %d", config.BLOCK_TIME-config.MINIBLOCK_HIGHDIFF+1, len(bl.MiniBlocks))
		return
	}

	// check whether the genesis blocks are all equal
	for _, mbl := range bl.MiniBlocks {

		if bl.Height != mbl.Height {
			return fmt.Errorf("MiniBlock has invalid height block height %d mbl height %d", bl.Height, mbl.Height)
		}
		if len(bl.Tips) != int(mbl.PastCount) {
			return fmt.Errorf("MiniBlock has wrong number of tips")
		}
		if len(bl.Tips) == 0 {
			panic("all miniblocks genesis must point to tip")
		} else if len(bl.Tips) == 1 {
			if binary.BigEndian.Uint32(bl.Tips[0][:]) != mbl.Past[0] {
				return fmt.Errorf("MiniBlock has invalid tip")
			}
		} else {
			panic("we only support 1 tips")
		} /*else if len(bl.Tips) == 2 {
			if binary.BigEndian.Uint32(bl.Tips[0][:]) != mbl.Past[0] {
				return fmt.Errorf("MiniBlock has invalid tip")
			}
			if binary.BigEndian.Uint32(bl.Tips[1][:]) != mbl.Past[1] {
				return fmt.Errorf("MiniBlock has invalid tip")
			}
			if mbl.Past[0] == mbl.Past[1] {
				return fmt.Errorf("MiniBlock refers to same tip twice")
			}
		} else {
			panic("we only support 1 tips")
		}*/
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
		chain.Log_lock.Lock()
		defer chain.Log_lock.Unlock()

		now := time.Now().UTC()
		now_human := now.Format(time.UnixDate)
		now_unix := now.UnixMilli()
		if chain.Prev_block_time == 0 {
			chain.Prev_block_time = now_unix
		}
		block_time_diff := now_unix - chain.Prev_block_time
		chain.Prev_block_time = now_unix

		filename := "received_blocks.csv"
		//try to open file before writing into it. if it does not exist, later write header as first line
		_, err3 := ioutil.ReadFile(filename)
		// If the file doesn't exist, create it, or append to the file
		f, err2 := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err2 != nil {
			log.Fatal(err2)
		}
		defer func() {
			if err3 := f.Close(); err3 != nil {
				log.Fatal(err3)
			}
		}()
		if err3 != nil {
			if _, err := f.Write([]byte("chain_height,final,miner_address,unix_time,human_time,minis,diff_millis\n")); err != nil {
				log.Fatal(err)
			}
		}

		var coinbase string

		max_topo := chain.Load_TOPO_HEIGHT()
		if max_topo > 25 { // we can lag a bit here, basically atleast around 10 mins lag
			max_topo -= 25
		}

		toporecord, _ := chain.Store.Topo_store.Read(max_topo)
		ss, _ := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		balance_tree, err2 := ss.GetTree(config.BALANCE_TREE)
		if err2 != nil {
			panic(err2)
		}
		_, key_compressed, _, err2 := balance_tree.GetKeyValueFromHash(mbl.KeyHash[:16])

		//record_version, _ := chain.ReadBlockSnapshotVersion(bl.Tips[0])
		mbl_coinbase, _ := rpc.NewAddressFromCompressedKeys(key_compressed)

		//		mbl_coinbase, _ := chain.KeyHashConverToAddress(key_compressed, record_version)
		addr := mbl_coinbase.String()
		coinbase = addr

		keys := chain.MiniBlocks.GetAllKeys(int64(mbl.Height))
		minis := 0
		if len(keys) > 0 {
			minis = len(chain.MiniBlocks.GetAllMiniBlocks(keys[0]))
		}
		fmt.Printf("miniblock %s inserted successfully for miner %s, total %d\n", "", addr, minis)

		line := fmt.Sprintf("%d,%t,%s,%d,%s,%d,%d\n",
			mbl.Height, mbl.Final, coinbase, now_unix, now_human, minis, block_time_diff)
		if _, err := f.Write([]byte(line)); err != nil {
			log.Fatal(err)
		}

		chain.RPC_NotifyNewMiniBlock.L.Lock()
		chain.RPC_NotifyNewMiniBlock.Broadcast()
		chain.RPC_NotifyNewMiniBlock.L.Unlock()

		chain.flip_top()
	}
	return err, result
}
