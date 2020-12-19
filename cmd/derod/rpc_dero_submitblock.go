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

package main

import "fmt"
import "context"
import "encoding/hex"
import "runtime/debug"

import "github.com/deroproject/derohe/structures"

func (DERO_RPC_APIS) SubmitBlock(ctx context.Context, block_data [2]string) (result structures.SubmitBlock_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	block_data_bytes, err := hex.DecodeString(block_data[0])
	if err != nil {
		logger.Infof("Submitting block could not be decoded")
		return result, fmt.Errorf("Submitting block could not be decoded. err: %s", err)
	}

	hashing_blob, err := hex.DecodeString(block_data[1])
	if err != nil || len(block_data[1]) == 0 {
		logger.Infof("Submitting block hashing_blob could not be decoded")
		return result, fmt.Errorf("block hashing blob could not be decoded. err: %s", err)
	}

	blid, sresult, err := chain.Accept_new_block(block_data_bytes, hashing_blob)

	if sresult {
		logger.Infof("Submitted block %s accepted", blid)
		return structures.SubmitBlock_Result{
			BLID:   blid.String(),
			Status: "OK",
		}, nil
	}

	if err != nil {
		logger.Infof("Submitting block %s err %s", blid, err)
		return result, err
	}

	logger.Infof("Submitting block rejected err %s", err)
	return structures.SubmitBlock_Result{
		Status: "REJECTED",
	}, nil

}
