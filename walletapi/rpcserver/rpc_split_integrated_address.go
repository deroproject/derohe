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

//import	"log"
//import 	"net/http"

import "github.com/deroproject/derohe/rpc"

//import "github.com/deroproject/derohe/rpc"

func SplitIntegratedAddress(ctx context.Context, p rpc.Split_Integrated_Address_Params) (result rpc.Split_Integrated_Address_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	if p.Integrated_Address == "" {
		return result, fmt.Errorf("Could not find integrated address as parameter")
	}

	addr, err := rpc.NewAddress(p.Integrated_Address)
	if err != nil {
		return result, fmt.Errorf("Error parsing integrated address err %s", err)
	}

	if !addr.IsDERONetwork() {
		return result, fmt.Errorf("integrated address  does not belong to DERO network")
	}

	if !addr.IsIntegratedAddress() {
		return result, fmt.Errorf("address %s is NOT an integrated address", addr.String())
	}
	result.Address = addr.BaseAddress().String()
	result.Payload_RPC = addr.Arguments
	return result, nil
}
