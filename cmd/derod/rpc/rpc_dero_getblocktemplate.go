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

package rpc

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/deroproject/derohe/rpc"
	"golang.org/x/time/rate"
)

// rate limiter is deployed, in case RPC is exposed over internet
// someone should not be just giving fake inputs and delay chain syncing
var get_block_limiter = rate.NewLimiter(16.0, 8) // 16 req per sec, burst of 8 req is okay

func GetBlockTemplate(ctx context.Context, p rpc.GetBlockTemplate_Params) (result rpc.GetBlockTemplate_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()
	/*
			if !get_block_limiter.Allow() { // if rate limiter allows, then add block to chain
				logger.Warnf("Too many get block template requests per sec rejected by chain.")

		                return nil,&jsonrpc.Error{
				Code:    jsonrpc.ErrorCodeInvalidRequest,
				Message: "Too many get block template requests per sec rejected by chain.",
			}


			}
	*/

	// validate address
	miner_address, err := rpc.NewAddress(p.Wallet_Address)
	if err != nil {
		return result, fmt.Errorf("Address could not be parsed, err:%s", err)
	}

	bl, mbl, mbl_hex, reserved_pos, err := chain.Create_new_block_template_mining(*miner_address)
	_ = mbl
	_ = reserved_pos
	if err != nil {
		return
	}

	prev_hash := ""
	for i := range bl.Tips {
		prev_hash = prev_hash + bl.Tips[i].String()
	}

	result.JobID = fmt.Sprintf("%d.%d.%s", bl.Timestamp, 0, p.Miner)
	if p.Block {
		result.Blocktemplate_blob = fmt.Sprintf("%x", bl.Serialize())
	}
	diff := chain.Get_Difficulty_At_Tips(bl.Tips)
	result.Blockhashing_blob = mbl_hex
	result.Height = bl.Height
	result.Prev_Hash = prev_hash
	result.Difficultyuint64 = diff.Uint64()
	result.Difficulty = diff.String()
	result.Status = "OK"

	return result, nil
}
