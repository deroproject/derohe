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

package p2p

import "github.com/deroproject/graviton"

// get parts of the specified balance tree chunk by chunk
func (c *Connection) TreeSection(request Request_Tree_Section_Struct, response *Response_Tree_Section_Struct) (err error) {
	defer handle_connection_panic(c)
	if request.Topo < 2 || request.SectionLength > 256 || len(request.Section) < int(request.SectionLength/8) { // we are expecting 1 block or 1 tx
		c.logger.V(1).Info("malformed object request  received, banning peer", "request", request)
		c.exit()
	}

	c.update(&request.Common) // update common information

	topo_sr, err := chain.Store.Topo_store.Read(request.Topo)
	if err != nil {
		return
	}

	{ // do the heavy lifting, merge all changes before this topoheight
		var topo_ss *graviton.Snapshot
		var topo_balance_tree *graviton.Tree
		if topo_ss, err = chain.Store.Balance_store.LoadSnapshot(topo_sr.State_Version); err == nil {
			if topo_balance_tree, err = topo_ss.GetTree(string(request.TreeName)); err == nil {
				cursor := topo_balance_tree.Cursor()
				response.KeyCount = topo_balance_tree.KeyCountEstimate()
				for k, v, err := cursor.SpecialFirst(request.Section, uint(request.SectionLength)); err == nil; k, v, err = cursor.Next() {
					response.Keys = append(response.Keys, k)
					response.Values = append(response.Values, v)

					if len(response.Keys) > 10000 {
						break
					}

				}
				err = nil

			}

		}

		if err != nil {
			return
		}

	}

	// if everything is OK, we must respond with object response
	fill_common(&response.Common) // fill common info
	return nil

}
