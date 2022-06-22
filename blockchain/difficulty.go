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
import "math"
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

const E = float64(2.71828182845905)

// hard code datatypes
func Diff(solvetime, blocktime, M int64, prev_diff int64) (diff int64) {
	if blocktime <= 0 || solvetime <= 0 || M <= 0 {
		panic("invalid parameters")
	}
	easypart := int64(math.Pow(E, ((1-float64(solvetime)/float64(blocktime))/float64(M))) * 10000)
	diff = (prev_diff * easypart) / 10000
	return diff
}

// big int implementation
func DiffBig(solvetime, blocktime, M int64, prev_diff *big.Int) (diff *big.Int) {
	if blocktime <= 0 || solvetime <= 0 || M <= 0 {
		panic("invalid parameters")
	}

	easypart := int64(math.Pow(E, ((1-float64(solvetime)/float64(blocktime))/float64(M))) * 10000)
	diff = new(big.Int).Mul(prev_diff, new(big.Int).SetInt64(easypart))
	diff.Div(diff, new(big.Int).SetUint64(10000))
	return diff
}

// when creating a new block, current_time in utc + chain_block_time must be added
// while verifying the block, expected time stamp should be replaced from what is in blocks header
// in DERO atlantis difficulty is based on previous tips
// get difficulty at specific  tips,
// algorithm is agiven above
// this should be more thoroughly evaluated

// NOTE: we need to evaluate if the mining adversary gains something, if the they set the time diff to 1
// we need to do more simulations and evaluations
// difficulty is now processed at sec level, mean how many hashes are require per sec to reach block time
// basica
func (chain *Blockchain) Get_Difficulty_At_Tips(tips []crypto.Hash) *big.Int {

	tips_string := ""
	for _, tip := range tips {
		tips_string += fmt.Sprintf("%s", tip.String())
	}

	if diff_bytes, found := chain.cache_Get_Difficulty_At_Tips.Get(tips_string); found {
		return new(big.Int).SetBytes([]byte(diff_bytes.(string)))
	}

	difficulty := Get_Difficulty_At_Tips(chain, tips)

	if chain.cache_enabled {
		chain.cache_Get_Difficulty_At_Tips.Add(tips_string, string(difficulty.Bytes())) // set in cache
	}
	return difficulty
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

	if mbl.HighDiff {
		block_difficulty.Mul(block_difficulty, new(big.Int).SetUint64(config.MINIBLOCK_HIGHDIFF))
	}

	if CheckPowHashBig(PoW, block_difficulty) == true {
		if chain.cache_enabled {
			chain.cache_IsMiniblockPowValid.Add(fmt.Sprintf("%s", cachekey), true) // set in cache
		}
		return true
	}
	return false
}

type DiffProvider interface {
	Load_Block_Height(crypto.Hash) int64
	Load_Block_Difficulty(crypto.Hash) *big.Int
	Load_Block_Timestamp(crypto.Hash) uint64
	Get_Block_Past(crypto.Hash) []crypto.Hash
}

func Get_Difficulty_At_Tips(source DiffProvider, tips []crypto.Hash) *big.Int {
	var MinimumDifficulty *big.Int
	GenesisDifficulty := new(big.Int).SetUint64(1)

	if globals.IsMainnet() {
		MinimumDifficulty = new(big.Int).SetUint64(config.Settings.MAINNET_MINIMUM_DIFFICULTY) // this must be controllable parameter
		GenesisDifficulty = new(big.Int).SetUint64(config.Settings.MAINNET_BOOTSTRAP_DIFFICULTY)
	} else {
		MinimumDifficulty = new(big.Int).SetUint64(config.Settings.TESTNET_MINIMUM_DIFFICULTY) // this must be controllable parameter
		GenesisDifficulty = new(big.Int).SetUint64(config.Settings.TESTNET_BOOTSTRAP_DIFFICULTY)
	}

	if chain, ok := source.(*Blockchain); ok {
		if chain.simulator == true {
			return new(big.Int).SetUint64(1)
		}
	}

	if len(tips) == 0 {
		return GenesisDifficulty
	}

	height := int64(0)
	for i := range tips {
		past_height := source.Load_Block_Height(tips[i])
		if past_height < 0 {
			panic(fmt.Errorf("could not find height for blid %s", tips[i]))
		}
		if height <= past_height {
			height = past_height
		}
	}
	height++
	//above height code is equivalent to below code
	//height := chain.Calculate_Height_At_Tips(tips)

	// until we have atleast 2 blocks, we cannot run the algo
	if height < 3 {
		return GenesisDifficulty
	}

	// after thr HF keep difficulty in control
	if height >= globals.Config.MAJOR_HF2_HEIGHT && height <= (globals.Config.MAJOR_HF2_HEIGHT+2) {
		return GenesisDifficulty
	}

	tip_difficulty := source.Load_Block_Difficulty(tips[0])
	tip_time := source.Load_Block_Timestamp(tips[0])

	parents := source.Get_Block_Past(tips[0])
	parent_time := source.Load_Block_Timestamp(parents[0])

	block_time := int64(config.BLOCK_TIME_MILLISECS)
	solve_time := int64(tip_time - parent_time)

	if solve_time > (block_time * 2) { // there should not be sudden decreases
		solve_time = block_time * 2
	}

	M := int64(8)
	difficulty := DiffBig(solve_time, block_time, M, tip_difficulty)

	if difficulty.Cmp(MinimumDifficulty) < 0 { // we can never be below minimum difficulty
		difficulty.Set(MinimumDifficulty)
	}

	return difficulty
}
