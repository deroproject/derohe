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
	"encoding/hex"
	"fmt"
	"runtime/debug"

	"github.com/deroproject/derohe/rpc"
)

func SubmitBlock(ctx context.Context, p rpc.SubmitBlock_Params) (result rpc.SubmitBlock_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	mbl_block_data_bytes, err := hex.DecodeString(p.MiniBlockhashing_blob)
	if err != nil {
		//logger.Info("Submitting block could not be decoded")
		return result, fmt.Errorf("Submitted block could not be decoded. err: %s", err)
	}

	var tstamp, extra uint64
	fmt.Sscanf(p.JobID, "%d.%d", &tstamp, &extra)

	mblid, blid, sresult, err := chain.Accept_new_block(tstamp, mbl_block_data_bytes)

	_ = mblid
	if sresult {
		//logger.Infof("Submitted block %s accepted", blid)

		result.JobID = p.JobID
		result.Status = "OK"
		result.MiniBlock = blid.IsZero()
		result.MBLID = mblid.String()
		if !result.MiniBlock {
			result.BLID = blid.String()
		}
		return result, nil
	}

	logger.V(1).Error(err, "Submitting block", "jobid", p.JobID)

	return rpc.SubmitBlock_Result{
		Status: "REJECTED",
	}, err

}
