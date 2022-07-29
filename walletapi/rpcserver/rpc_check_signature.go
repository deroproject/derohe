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

import (
	"context"
	"encoding/hex"
	"fmt"
	"runtime/debug"

	"github.com/deroproject/derohe/rpc"
)

func CheckSignature(ctx context.Context, p rpc.Check_Signature) (result rpc.Check_Signature_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	if p.Data == "" {
		return result, fmt.Errorf("Data field is empty")
	}

	decData, err := hex.DecodeString(p.Data)
	if err != nil {
		return result, fmt.Errorf("Error parsing data err %s", err)
	}
	length := len(decData)

	w := fromContext(ctx)
	signer, message, err := w.wallet.CheckSignature(decData[:length])
	if err != nil {
		return result, fmt.Errorf("Error checking signature err %s", err)
	}
	if p.Plain {
		result.Message = string(message[:])
	} else {
		result.Message = hex.EncodeToString(message[:])
	}
	result.Signer = signer.String()

	return result, nil
}
