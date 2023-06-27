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
	"encoding/base64"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/dvm"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/graviton"
)

//import "encoding/binary"

//import "github.com/deroproject/derohe/config"

//import "github.com/deroproject/derohe/transaction"
//import "github.com/deroproject/derohe/blockchain"

func GetGasEstimate(ctx context.Context, p rpc.GasEstimate_Params) (result rpc.GasEstimate_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace r %s %s", r, debug.Stack())
		}
	}()

	if len(p.SC_Code) >= 1 && !strings.Contains(strings.ToLower(p.SC_Code), "initialize") { // decode SC from base64 if possible, since json hash limitations
		if sc, err := base64.StdEncoding.DecodeString(p.SC_Code); err == nil {
			p.SC_Code = string(sc)
		}
	}

	var signer *rpc.Address

	if len(p.Signer) > 0 {
		if signer, err = rpc.NewAddress(p.Signer); err != nil {
			return
		}
	}

	incoming_values := map[crypto.Hash]uint64{}
	for _, t := range p.Transfers {
		if t.Burn > 0 {
			incoming_values[t.SCID] += t.Burn
		}
	}

	toporecord, err := chain.Store.Topo_store.Read(chain.Load_TOPO_HEIGHT())
	// we must now fill in compressed ring members
	if err == nil {
		var ss *graviton.Snapshot
		ss, err = chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		if err == nil {
			s := dvm.SimulatorInitialize(ss)
			if len(p.SC_Code) >= 1 { // we need to install the SC
				if _, result.GasCompute, result.GasStorage, err = s.SCInstall(p.SC_Code, incoming_values, p.SC_RPC, signer, 0); err != nil {
					return
				}
			} else { // we need to estimate gas for already installed contract
				if result.GasCompute, result.GasStorage, err = s.RunSC(incoming_values, p.SC_RPC, signer, 0); err != nil {
					return
				}
			}
		}
	}

	//fmt.Printf("p %+v\n", p)

	result.Status = "OK"
	err = nil

	//logger.Debugf("result %+v\n", result);
	return
}
