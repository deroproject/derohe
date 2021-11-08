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

import "fmt"
import "context"
import "runtime/debug"
import "github.com/deroproject/derohe/rpc"

func GetBlockHeaderByTopoHeight(ctx context.Context, p rpc.GetBlockHeaderByTopoHeight_Params) (result rpc.GetBlockHeaderByHeight_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	if int64(p.TopoHeight) > chain.Load_TOPO_HEIGHT() {
		err = fmt.Errorf("Too big topo height: %d, current blockchain height = %d", p.TopoHeight, chain.Load_TOPO_HEIGHT())
		return
	}

	//return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("NOT SUPPORTED height: %d, current blockchain height = %d", p.Height, chain.Get_Height())}
	hash, err := chain.Load_Block_Topological_order_at_index(int64(p.TopoHeight))
	if err != nil { // if err return err
		err = fmt.Errorf("User requested %d height block, chain topo height %d but err occured %s", p.TopoHeight, chain.Load_TOPO_HEIGHT(), err)
		return
	}

	block_header, err := chain.GetBlockHeader(hash)
	if err != nil { // if err return err
		err = fmt.Errorf("User requested %d height block, chain  topo height %d but err occured %s", p.TopoHeight, chain.Load_TOPO_HEIGHT(), err)
		return
	}

	return rpc.GetBlockHeaderByHeight_Result{ // return success
		Block_Header: block_header,
		Status:       "OK",
	}, nil

}
