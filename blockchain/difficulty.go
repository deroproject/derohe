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
import "math/big"

import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/globals"

var (
	// bigZero is 0 represented as a big.Int.  It is defined here to avoid
	// the overhead of creating it multiple times.
	bigZero = big.NewInt(0)

	// bigOne is 1 represented as a big.Int.  It is defined here to avoid
	// the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	// oneLsh256 is 1 shifted left 256 bits.  It is defined here to avoid
	// the overhead of creating it multiple times.
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)

	// enabling this will simulation mode with hard coded difficulty set to 1
	// the variable is knowingly not exported, so no one can tinker with it
	//simulation = false // simulation mode is disabled
)

// HashToBig converts a PoW has into a big.Int that can be used to
// perform math comparisons.
func HashToBig(buf crypto.Hash) *big.Int {
	// A Hash is in little-endian, but the big package wants the bytes in
	// big-endian, so reverse them.
	blen := len(buf) // its hardcoded 32 bytes, so why do len but lets do it
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}

	return new(big.Int).SetBytes(buf[:])
}

// this function calculates the difficulty in big num form
func ConvertDifficultyToBig(difficultyi uint64) *big.Int {
	if difficultyi == 0 {
		panic("difficulty can never be zero")
	}
	// (1 << 256) / (difficultyNum )
	difficulty := new(big.Int).SetUint64(difficultyi)
	denominator := new(big.Int).Add(difficulty, bigZero) // above 2 lines can be merged
	return new(big.Int).Div(oneLsh256, denominator)
}

func ConvertIntegerDifficultyToBig(difficultyi *big.Int) *big.Int {
	if difficultyi.Cmp(bigZero) == 0 {
		panic("difficulty can never be zero")
	}
	return new(big.Int).Div(oneLsh256, difficultyi)
}

// this function check whether the pow hash meets difficulty criteria
func CheckPowHash(pow_hash crypto.Hash, difficulty uint64) bool {
	big_difficulty := ConvertDifficultyToBig(difficulty)
	big_pow_hash := HashToBig(pow_hash)

	if big_pow_hash.Cmp(big_difficulty) <= 0 { // if work_pow is less than difficulty
		return true
	}
	return false
}

// this function check whether the pow hash meets difficulty criteria
// however, it take diff in  bigint format
func CheckPowHashBig(pow_hash crypto.Hash, big_difficulty_integer *big.Int) bool {
	big_pow_hash := HashToBig(pow_hash)

	big_difficulty := ConvertIntegerDifficultyToBig(big_difficulty_integer)
	if big_pow_hash.Cmp(big_difficulty) <= 0 { // if work_pow is less than difficulty
		return true
	}
	return false
}

// when creating a new block, current_time in utc + chain_block_time must be added
// while verifying the block, expected time stamp should be replaced from what is in blocks header
// in DERO atlantis difficulty is based on previous tips
// get difficulty at specific  tips,
// algorithm is as follows choose biggest difficulty tip (// division is integer and not floating point)
// diff = (parent_diff +   (parent_diff / 100 * max(1 - (parent_timestamp - parent_parent_timestamp) // (chain_block_time*2//3), -1))
// this should be more thoroughly evaluated

// NOTE: we need to evaluate if the mining adversary gains something, if the they set the time diff to 1
// we need to do more simulations and evaluations
// difficulty is now processed at sec level, mean how many hashes are require per sec to reach block time
func (chain *Blockchain) Get_Difficulty_At_Tips(tips []crypto.Hash) *big.Int {

	tips_string := ""
	for _, tip := range tips {
		tips_string += fmt.Sprintf("%s", tip.String())
	}

	if diff_bytes, found := chain.cache_Get_Difficulty_At_Tips.Get(tips_string); found {
		return new(big.Int).SetBytes([]byte(diff_bytes.(string)))
	}

	var MinimumDifficulty *big.Int
	change := new(big.Int)
	step := new(big.Int)

	if globals.IsMainnet() {
		MinimumDifficulty = new(big.Int).SetUint64(config.MAINNET_MINIMUM_DIFFICULTY) // this must be controllable parameter
	} else {
		MinimumDifficulty = new(big.Int).SetUint64(config.TESTNET_MINIMUM_DIFFICULTY) // this must be controllable parameter
	}
	GenesisDifficulty := new(big.Int).SetUint64(1)

	if len(tips) == 0 || chain.simulator == true {
		return GenesisDifficulty
	}

	height := chain.Calculate_Height_At_Tips(tips)

	// hard fork version 1 has difficulty set to  1
	/*if 1 == chain.Get_Current_Version_at_Height(height) {
		return new(big.Int).SetUint64(1)
	}*/

	/*
	   	// if we are hardforking from 1 to 2
	   	// we can start from high difficulty to find the right point
	   	if height >= 1 && chain.Get_Current_Version_at_Height(height-1) == 1 && chain.Get_Current_Version_at_Height(height) == 2 {
	   		if globals.IsMainnet() {
	   			bootstrap_difficulty := new(big.Int).SetUint64(config.MAINNET_BOOTSTRAP_DIFFICULTY) // return bootstrap mainnet difficulty
	   			rlog.Infof("Returning bootstrap difficulty %s at height %d", bootstrap_difficulty.String(), height)
	   			return bootstrap_difficulty
	   		} else {
	   			bootstrap_difficulty := new(big.Int).SetUint64(config.TESTNET_BOOTSTRAP_DIFFICULTY)
	   			rlog.Infof("Returning bootstrap difficulty %s at height %d", bootstrap_difficulty.String(), height)
	   			return bootstrap_difficulty // return bootstrap difficulty for testnet
	   		}
	   	}

	       // if we are hardforking from 3 to 4
	   	// we can start from high difficulty to find the right point
	   	if height >= 1 && chain.Get_Current_Version_at_Height(height-1) <= 3 && chain.Get_Current_Version_at_Height(height) == 4 {
	   		if globals.IsMainnet() {
	   			bootstrap_difficulty := new(big.Int).SetUint64(config.MAINNET_BOOTSTRAP_DIFFICULTY_hf4) // return bootstrap mainnet difficulty
	   			rlog.Infof("Returning bootstrap difficulty %s at height %d", bootstrap_difficulty.String(), height)
	   			return bootstrap_difficulty
	   		} else {
	   			bootstrap_difficulty := new(big.Int).SetUint64(config.TESTNET_BOOTSTRAP_DIFFICULTY)
	   			rlog.Infof("Returning bootstrap difficulty %s at height %d", bootstrap_difficulty.String(), height)
	   			return bootstrap_difficulty // return bootstrap difficulty for testnet
	   		}
	   	}

	*/

	// until we have atleast 2 blocks, we cannot run the algo
	if height < 3 && chain.Get_Current_Version_at_Height(height) <= 1 {
		return MinimumDifficulty
	}

	//  take the time from the most heavy block

	biggest_difficulty := chain.Load_Block_Difficulty(tips[0])
	parent_highest_time := chain.Load_Block_Timestamp(tips[0])

	// find parents parents tip from the most heavy block's parent
	parent_past := chain.Get_Block_Past(tips[0])
	past_biggest_tip := parent_past[0]
	parent_parent_highest_time := chain.Load_Block_Timestamp(past_biggest_tip)

	if biggest_difficulty.Cmp(MinimumDifficulty) < 0 {
		biggest_difficulty.Set(MinimumDifficulty)
	}

	block_time := config.BLOCK_TIME_MILLISECS
	step.Div(biggest_difficulty, new(big.Int).SetUint64(100))

	// create 3 ranges, used for physical verification
	switch {
	case (parent_highest_time - parent_parent_highest_time) <= block_time-1000: // increase diff
		change.Add(change, step) // block was found earlier, increase diff

	case (parent_highest_time - parent_parent_highest_time) >= block_time+1000: // decrease diff
		change.Sub(change, step) // block was found late, decrease diff
		change.Sub(change, step)

	default: //  if less than 1 sec deviation,use previous diff, ie change is zero

	}

	biggest_difficulty.Add(biggest_difficulty, change)

	if biggest_difficulty.Cmp(MinimumDifficulty) < 0 { // we can never be below minimum difficulty
		biggest_difficulty.Set(MinimumDifficulty)
	}

	if !chain.cache_disabled {
		chain.cache_Get_Difficulty_At_Tips.Add(tips_string, string(biggest_difficulty.Bytes())) // set in cache
	}
	return biggest_difficulty
}

func (chain *Blockchain) VerifyMiniblockPoW(bl *block.Block, mbl block.MiniBlock) bool {
	var cachekey []byte
	for i := range bl.Tips {
		cachekey = append(cachekey, bl.Tips[i][:]...)
	}
	cachekey = append(cachekey, mbl.Serialize()...)
	if _, ok := chain.cache_IsMiniblockPowValid.Get(fmt.Sprintf("%s", cachekey)); ok {
		return true
	}

	PoW := mbl.GetPoWHash()
	block_difficulty := chain.Get_Difficulty_At_Tips(bl.Tips)

	// test new difficulty checksm whether they are equivalent to integer math
	/*if CheckPowHash(PoW, block_difficulty.Uint64()) != CheckPowHashBig(PoW, block_difficulty) {
		logger.Panicf("Difficuly mismatch between big and uint64 diff ")
	}*/

	if CheckPowHashBig(PoW, block_difficulty) == true {
		if !chain.cache_disabled {
			chain.cache_IsMiniblockPowValid.Add(fmt.Sprintf("%s", cachekey), true) // set in cache
		}
		return true
	}
	return false
}

// this function calculates difficulty on the basis of previous difficulty  and number of blocks
// THIS is the ideal algorithm for us as it will be optimal based on the number of orphan blocks
// we may deploy it when the  block reward becomes insignificant in comparision to fees
//  basically tail emission kicks in or we need to optimally increase number of blocks
// the algorithm does NOT work if the network has a single miner  !!!
// this algorithm will work without the concept of time
