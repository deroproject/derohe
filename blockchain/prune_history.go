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

package blockchain

// this file will prune the history of the blockchain, making it light weight
// the pruner works like this
// identify a point in history before which all history is discarded
// the entire thing works cryptographically and thus everything is cryptographically verified
// this function is the only one which does not work in append-only

import "os"
import "fmt"
import "path/filepath"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/graviton"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/globals"

const CHUNK_SIZE = 100000 // write 100000 account chunks, actually we should be writing atleast 100,000 accounts

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func DirSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	_ = err
	return size
}

func Prune_Blockchain(prune_topo int64) (err error) {

	var store storage

	// initialize store

	current_path := filepath.Join(globals.GetDataDirectory())

	if store.Balance_store, err = graviton.NewDiskStore(filepath.Join(current_path, "balances")); err == nil {
		if err = store.Topo_store.Open(current_path); err == nil {
			store.Block_tx_store.basedir = current_path
		} else {
			return err
		}
	}

	max_topoheight := store.Topo_store.Count()
	for ; max_topoheight >= 0; max_topoheight-- {
		if toporecord, err := store.Topo_store.Read(max_topoheight); err == nil {
			if !toporecord.IsClean() {
				break
			}
		}
	}

	//prune_topoheight := max_topoheight - 97
	prune_topoheight := prune_topo
	if max_topoheight-prune_topoheight < 50 {
		return fmt.Errorf("We need atleast 50 blocks to prune")
	}

	err = rewrite_graviton_store(&store, prune_topoheight, max_topoheight)

	discard_blocks_and_transactions(&store, prune_topoheight)

	// close original store and move new store in the same place
	store.Balance_store.Close()
	old_path := filepath.Join(current_path, "balances")
	new_path := filepath.Join(current_path, "balances_new")

	globals.Logger.Info("Old balance tree", "size", ByteCountIEC(DirSize(old_path)))
	globals.Logger.Info("balance tree after pruning history", "size", ByteCountIEC(DirSize(new_path)))
	os.RemoveAll(old_path)
	return os.Rename(new_path, old_path)

}

// first lets free space by discarding blocks, and txs before the historical point
// any error while deleting should be considered non fatal
func discard_blocks_and_transactions(store *storage, topoheight int64) {

	globals.Logger.Info("Block store before pruning", "size", ByteCountIEC(DirSize(filepath.Join(store.Block_tx_store.basedir, "bltx_store"))))

	for i := int64(0); i < topoheight-20; i++ { // donot some more blocks for sanity currently
		if toporecord, err := store.Topo_store.Read(i); err == nil {
			blid := toporecord.BLOCK_ID

			var bl block.Block
			if block_data, err := store.Block_tx_store.ReadBlock(blid); err == nil {
				if err = bl.Deserialize(block_data); err == nil { // we should deserialize the block here
					for _, txhash := range bl.Tx_hashes { // we also have to purge the tx hashes
						_ = store.Block_tx_store.DeleteTX(txhash) // delete tx hashes
						//fmt.Printf("DeleteTX %x\n", txhash)
					}
				}

				// lets delete the block data also
				_ = store.Block_tx_store.DeleteBlock(blid)
				//fmt.Printf("DeleteBlock %x\n", blid)
			}
		}
	}
	globals.Logger.Info("Block store after pruning ", "size", ByteCountIEC(DirSize(filepath.Join(store.Block_tx_store.basedir, "bltx_store"))))
}

// this will rewrite the graviton store
func rewrite_graviton_store(store *storage, prune_topoheight int64, max_topoheight int64) (err error) {
	var write_store *graviton.Store
	writebalancestorepath := filepath.Join(store.Block_tx_store.basedir, "balances_new")

	if write_store, err = graviton.NewDiskStore(writebalancestorepath); err != nil {
		return err
	}

	toporecord, err := store.Topo_store.Read(prune_topoheight)
	if err != nil {
		return err
	}

	var major_copy uint64

	{ // do the heavy lifting, merge all changes before this topoheight
		var old_ss, write_ss *graviton.Snapshot
		var old_balance_tree, write_balance_tree *graviton.Tree
		if old_ss, err = store.Balance_store.LoadSnapshot(toporecord.State_Version); err == nil {
			if old_balance_tree, err = old_ss.GetTree(config.BALANCE_TREE); err == nil {
				if write_ss, err = write_store.LoadSnapshot(0); err == nil {
					if write_balance_tree, err = write_ss.GetTree(config.BALANCE_TREE); err == nil {

						var latest_commit_version uint64
						latest_commit_version, err = clone_entire_tree(old_balance_tree, write_balance_tree)
						//fmt.Printf("cloned entire tree version %d err '%s'\n", latest_commit_version, err)
						major_copy = latest_commit_version
					}
				}
			}
		}

		if err != nil {
			return err
		}
	}

	// now we must do block to block changes till the top block
	{

		var new_entries []int64
		var commit_versions []uint64

		for i := prune_topoheight; i < max_topoheight; i++ {
			var old_toporecord, new_toporecord TopoRecord
			var old_ss, new_ss, write_ss *graviton.Snapshot
			var old_balance_tree, new_balance_tree, write_tree *graviton.Tree

			// fetch old tree data
			old_topo := i
			new_topo := i + 1

			err = nil

			if old_toporecord, err = store.Topo_store.Read(old_topo); err == nil {
				if old_ss, err = store.Balance_store.LoadSnapshot(old_toporecord.State_Version); err == nil {
					if old_balance_tree, err = old_ss.GetTree(config.BALANCE_TREE); err == nil {

						// fetch new tree data
						if new_toporecord, err = store.Topo_store.Read(new_topo); err == nil {
							if new_ss, err = store.Balance_store.LoadSnapshot(new_toporecord.State_Version); err == nil {
								if new_balance_tree, err = new_ss.GetTree(config.BALANCE_TREE); err == nil {

									// fetch tree where to write it
									if write_ss, err = write_store.LoadSnapshot(0); err == nil {
										if write_tree, err = write_ss.GetTree(config.BALANCE_TREE); err == nil {

											//   	new_balance_tree.Graph("/tmp/original.dot")
											//   	write_tree.Graph("/tmp/writable.dot")
											//   	fmt.Printf("writing new graph\n")

											latest_commit_version, err := clone_tree_changes(old_balance_tree, new_balance_tree, write_tree)
											logger.Info(fmt.Sprintf("cloned tree changes from %d(%d)  to %d(%d) , wrote version %d err '%s'", old_topo, old_toporecord.State_Version, new_topo, new_toporecord.State_Version, latest_commit_version, err))

											new_entries = append(new_entries, new_topo)
											commit_versions = append(commit_versions, latest_commit_version)

											if write_hash, err := write_tree.Hash(); err == nil {
												if new_hash, err := new_balance_tree.Hash(); err == nil {

													// if this ever  fails, means we have somthing nasty going on
													// maybe  graviton or some disk corruption
													if new_hash != write_hash {
														fmt.Printf("wrt %x \nnew %x  \n", write_hash, new_hash)
														panic("corruption")
													}

												}
											}

										} else {
											//fmt.Printf("err  from graviton internal  %s\n", err)
											return err // this is irrepairable  damage
										}
									}
								}
							}
						}
					}
				}
			}

			if err != nil {
				//fmt.Printf("err  from gravitonnnnnnnnnn  %s\n", err)
				return err
			}
		}

		// now lets store all the commit versions in 1 go
		for i, topo := range new_entries {
			if old_toporecord, err := store.Topo_store.Read(topo); err == nil {
				//fmt.Printf("writing toporecord %d version %d\n",topo, commit_versions[i])
				store.Topo_store.Write(topo, old_toporecord.BLOCK_ID, commit_versions[i], old_toporecord.Height)
			} else {
				fmt.Printf("err reading/writing toporecord %d %s\n", topo, err)
			}
		}

	}

	// now overwrite the starting topo mapping
	for i := int64(0); i <= prune_topoheight; i++ { // overwrite the entries in the topomap
		if toporecord, err := store.Topo_store.Read(i); err == nil {
			store.Topo_store.Write(i, toporecord.BLOCK_ID, major_copy, toporecord.Height)
		} else {
			fmt.Printf("err writing toporecord  %s\n", err)
			return err // this is irrepairable  damage
		}
	}

	// now lets remove the old graviton db
	write_store.Close()

	return

}

// clone tree changes between 2 versions (old_tree, new_tree and then commit them to write_tree)
func clone_tree_changes(old_tree, new_tree, write_tree *graviton.Tree) (latest_commit_version uint64, err error) {
	if old_tree.IsDirty() || new_tree.IsDirty() || write_tree.IsDirty() {
		panic("trees cannot be dirty")
	}
	insert_count := 0
	modify_count := 0
	insert_handler := func(k, v []byte) {
		insert_count++
		//fmt.Printf("insert %x %x\n",k,v)
		write_tree.Put(k, v)
	}
	modify_handler := func(k, v []byte) { // modification receives old value
		modify_count++

		new_value, _ := new_tree.Get(k)
		write_tree.Put(k, new_value)
	}

	graviton.Diff(old_tree, new_tree, nil, modify_handler, insert_handler)

	//fmt.Printf("insert count %d modify_count %d\n", insert_count, modify_count)
	if write_tree.IsDirty() {
		return graviton.Commit(write_tree)
	} else {
		return write_tree.GetVersion(), nil
	}

}

// clone entire tree in chunks
func clone_entire_tree(old_tree, new_tree *graviton.Tree) (latest_commit_version uint64, err error) {
	c := old_tree.Cursor()

	object_counter := int64(0)
	for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
		if object_counter != 0 && object_counter%CHUNK_SIZE == 0 {
			if latest_commit_version, err = graviton.Commit(new_tree); err != nil {
				fmt.Printf("err while cloingggggggggggg %s\n", err)
				return 0, err
			}
		}
		new_tree.Put(k, v)
		object_counter++
	}

	//if new_tree.IsDirty() {
	if latest_commit_version, err = graviton.Commit(new_tree); err != nil {
		fmt.Printf("err while cloingggggggggggg qqqqqqqqqqqq %s\n", err)
		return 0, err
	}

	//}

	/*old_hash,erro := old_tree.Hash()
	new_hash,errn := new_tree.Hash()
	fmt.Printf("old %x err %x\nmew %x err %s       \n", old_hash,erro,new_hash,errn )
	*/
	return latest_commit_version, nil
}
