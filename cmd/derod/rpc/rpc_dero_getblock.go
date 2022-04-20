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
	"fmt"
	"strconv"
)
import "context"
import "encoding/hex"
import "encoding/json"
import "runtime/debug"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/rpc"

//import "github.com/deroproject/derosuite/blockchain"

func GetBlock(ctx context.Context, p rpc.GetBlock_Params) (result rpc.GetBlock_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	var hash crypto.Hash

	if crypto.HashHexToHash(p.Hash) == hash { // user requested using height
		fmt.Println("requested block at height: " + strconv.FormatInt(int64(p.Height), 10))
		if int64(p.Height) > chain.Load_TOPO_HEIGHT() {
			err = fmt.Errorf("user requested block at toopheight more than chain topoheight")
			return
		}

		hash, err = chain.Load_Block_Topological_order_at_index(int64(p.Height))
		if err != nil { // if err return err
			return result, fmt.Errorf("User requested %d height block, chain height %d but err occured %s", p.Height, chain.Get_Height(), err)
		}

	} else {
		hash = crypto.HashHexToHash(p.Hash)
	}

	block_header, err := GetBlockHeader(chain, hash)
	if err != nil { // if err return err
		return
	}

	bl, err := chain.Load_BL_FROM_ID(hash)
	//for idx, mbl := range bl.MiniBlocks {
	//	fmt.Println("processing miniblock:" + strconv.Itoa(idx))
	//	//var ss *graviton.Snapshot
	//	max_topo := chain.Load_TOPO_HEIGHT()
	//	if max_topo > 25 { // we can lag a bit here, basically atleast around 10 mins lag
	//		max_topo -= 25
	//	}
	//
	//	toporecord, _ := chain.Store.Topo_store.Read(max_topo)
	//	ss, _ := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	//	balance_tree, err2 := ss.GetTree(config.BALANCE_TREE)
	//	if err2 != nil {
	//		panic(err2)
	//	}
	//	_, key_compressed, _, err2 := balance_tree.GetKeyValueFromHash(mbl.KeyHash[:16])
	//	if err2 != nil { // the full block does not have the hashkey based coinbase
	//		fmt.Println("miniblock has no hashkey: " + strconv.Itoa(idx))
	//		continue
	//	}
	//
	//	//record_version, _ := chain.ReadBlockSnapshotVersion(bl.Tips[0])
	//	mbl_coinbase, _ := rpc.NewAddressFromCompressedKeys(key_compressed)
	//
	//	//		mbl_coinbase, _ := chain.KeyHashConverToAddress(key_compressed, record_version)
	//	addr := mbl_coinbase.String()
	//	fmt.Println("Coinbase addr: " + addr)
	//	//record_version, _ := chain.ReadBlockSnapshotVersion(bl.Tips[0])
	//	//mbl_coinbase, _ := chain.KeyHashConverToAddress(mbl.KeyHash, record_version)
	//	//fmt.Printf("Coinbase addr: " + mbl_coinbase)
	//}
	if err != nil { // if err return err
		return
	}

	json_encoded_bytes, err := json.Marshal(bl)
	if err != nil { // if err return err
		return
	}
	return rpc.GetBlock_Result{ // return success
		Block_Header: block_header,
		Blob:         hex.EncodeToString(bl.Serialize()),
		Json:         string(json_encoded_bytes),
		Status:       "OK",
	}, nil
}
