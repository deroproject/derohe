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

package rpcserver

import "fmt"

import "context"
import "runtime/debug"
import "github.com/romana/rlog"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/cryptography/crypto"

func (w *WALLET_RPC_APIS) ScInvoke(ctx context.Context, p rpc.SC_Invoke_Params) (err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	rlog.Debugf("ScInvoke  handler")
	defer rlog.Debugf("ScInvoke  handler finished")

	if !w.wallet.GetMode() { // if wallet is in online mode, use the fees, provided by the daemon, else we need to use what is provided by the user
		return fmt.Errorf("Wallet is in offline mode")
	}

	// translate rpc to arguments

	//fmt.Printf("incoming transfer params %+v\n", p)

	if p.SC_ID == "" {
		return fmt.Errorf("SCID cannot be empty")
	}

	// if destination is "", we will choose a random address automatically

	var tp rpc.Transfer_Params
	tp.Transfers = append(tp.Transfers, rpc.Transfer{Destination: "deto1qxsplx7vzgydacczw6vnrtfh3fxqcjevyxcvlvl82fs8uykjkmaxgfgulfha5", Amount: 0, Burn: p.SC_DERO_Deposit})

	// we must burn this much tokens
	if p.SC_TOKEN_Deposit >= 1 {
		scid := crypto.HashHexToHash(p.SC_ID)
		tp.Transfers = append(tp.Transfers, rpc.Transfer{SCID: scid, Amount: 0, Burn: p.SC_TOKEN_Deposit})
	}
	tp.SC_RPC = p.SC_RPC
	tp.SC_ID = p.SC_ID

	//fmt.Printf("transfers %+v\n", tp)

	err = wallet_apis.Transfer(context.Background(), tp)

	//fmt.Printf("err transfer %s\n", err)

	return err

}
