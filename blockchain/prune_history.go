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
import "math/big"
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
	if err != nil {
		globals.Logger.Error(err, "error rewriting graviton store")
	}

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

	size_before := ByteCountIEC(DirSize(filepath.Join(store.Block_tx_store.basedir, "bltx_store")))

	for i := int64(0); i < topoheight-20; i++ { // donot some more blocks for sanity currently

		if i%1000 == 0 {
			globals.Logger.Info("Deleting old block/txs", "done", float64(i*100)/float64(topoheight-20))
		}

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
	globals.Logger.Info("Block store before pruning", "size", size_before)
	globals.Logger.Info("Block store after pruning ", "size", ByteCountIEC(DirSize(filepath.Join(store.Block_tx_store.basedir, "bltx_store"))))
}

// clone a snapshot, this is a dero arch dependent
// since the trees can be large in the long term, we do them in chunks
func clone_snapshot(rsource, wsource *graviton.Store, r_ssversion uint64) (latest_commit_version uint64, err error) {

	var old_ss, write_ss *graviton.Snapshot
	var old_balance_tree, write_balance_tree *graviton.Tree
	var old_meta_tree, write_meta_tree *graviton.Tree

	if old_ss, err = rsource.LoadSnapshot(r_ssversion); err != nil {
		return
	}

	if write_ss, err = wsource.LoadSnapshot(0); err != nil {
		return
	}

	if old_balance_tree, err = old_ss.GetTree(config.BALANCE_TREE); err != nil {
		return
	}

	if write_balance_tree, err = write_ss.GetTree(config.BALANCE_TREE); err != nil {
		return
	}

	{ // copy old tree to new tree, in chunks
		c := old_balance_tree.Cursor()
		object_counter := int64(0)
		for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
			if object_counter != 0 && object_counter%CHUNK_SIZE == 0 {
				if latest_commit_version, err = graviton.Commit(write_balance_tree); err != nil {
					fmt.Printf("err while cloingggggggggggg %s\n", err)
					return 0, err
				}
			}
			write_balance_tree.Put(k, v)
			object_counter++
		}
	}

	if latest_commit_version, err = graviton.Commit(write_balance_tree); err != nil {
		return 0, err
	}

	globals.Logger.Info("Main balance tree cloned")

	/*	h,_ := old_balance_tree.Hash()
		fmt.Printf("old balance hash %+v\n",h )
		h,_ = write_balance_tree.Hash()
		fmt.Printf("write balance hash %+v\n",h )

		//os.Exit(0)
	*/
	// copy meta tree for scid
	if old_meta_tree, err = old_ss.GetTree(config.SC_META); err != nil {
		return
	}
	if write_meta_tree, err = write_ss.GetTree(config.SC_META); err != nil {
		return
	}

	var sc_list [][]byte

	{ // copy sc tree, in chunks
		c := old_meta_tree.Cursor()

		object_counter := int64(0)
		for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
			if object_counter != 0 && object_counter%CHUNK_SIZE == 0 {
				if latest_commit_version, err = graviton.Commit(write_meta_tree); err != nil {
					fmt.Printf("err while cloingggggggggggg %s\n", err)
					return 0, err
				}
			}
			write_meta_tree.Put(k, v)
			sc_list = append(sc_list, k)
			object_counter++
		}
	}

	if latest_commit_version, err = graviton.Commit(write_meta_tree); err != nil {
		return 0, err
	}

	/*	h,_ = old_meta_tree.Hash()
		fmt.Printf("old meta hash %+v\n",h )
		h,_ = write_meta_tree.Hash()
		fmt.Printf("new meta hash %+v\n",h )

		os.Exit(0)
	*/
	sc_names := map[string]bool{}
	// now we have to copy all scs data one by one
	for _, scid := range sc_list {
		var old_sc_tree, write_sc_tree *graviton.Tree
		if old_sc_tree, err = old_ss.GetTree(string(scid)); err != nil {
			return
		}
		if write_sc_tree, err = write_ss.GetTree(string(scid)); err != nil {
			return
		}
		c := old_sc_tree.Cursor()
		for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
			write_sc_tree.Put(k, v)
		}
		sc_names[string(scid)] = true
		if latest_commit_version, err = graviton.Commit(write_sc_tree); err != nil {
			return
		}
	}
	globals.Logger.Info("SCs cloned")

	return
}

// diff a snapshot from block to block, this is a dero arch dependent
// entire block is done in a single commit
func diff_snapshot(rsource, wsource *graviton.Store, old_version uint64, new_version uint64) (latest_commit_version uint64, err error) {

	var sc_trees []*graviton.Tree
	var old_ss, new_ss, write_ss *graviton.Snapshot
	var old_tree, new_tree, write_tree *graviton.Tree

	if old_ss, err = rsource.LoadSnapshot(old_version); err != nil {
		return
	}

	if new_ss, err = rsource.LoadSnapshot(new_version); err != nil {
		return
	}
	if write_ss, err = wsource.LoadSnapshot(0); err != nil {
		return
	}

	if old_tree, err = old_ss.GetTree(config.BALANCE_TREE); err != nil {
		return
	}
	if new_tree, err = new_ss.GetTree(config.BALANCE_TREE); err != nil {
		return
	}
	if write_tree, err = write_ss.GetTree(config.BALANCE_TREE); err != nil {
		return
	}

	// diff and update balance tree
	clone_tree_changes(old_tree, new_tree, write_tree)
	sc_trees = append(sc_trees, write_tree)

	// copy meta tree for scid
	if old_tree, err = old_ss.GetTree(config.SC_META); err != nil {
		return
	}
	if new_tree, err = new_ss.GetTree(config.SC_META); err != nil {
		return
	}
	if write_tree, err = write_ss.GetTree(config.SC_META); err != nil {
		return
	}

	var sc_list_new, sc_list_modified [][]byte

	// diff and update meta tree
	{
		insert_handler := func(k, v []byte) {
			write_tree.Put(k, v)
			sc_list_new = append(sc_list_new, k)
		}
		modify_handler := func(k, v []byte) { // modification receives old value
			new_value, _ := new_tree.Get(k)
			write_tree.Put(k, new_value)
			sc_list_modified = append(sc_list_modified, k)
		}

		graviton.Diff(old_tree, new_tree, nil, modify_handler, insert_handler)
	}

	sc_trees = append(sc_trees, write_tree)

	// now we have to copy new scs data one by one
	for _, scid := range sc_list_new {
		if old_tree, err = old_ss.GetTree(string(scid)); err != nil {
			return
		}
		if new_tree, err = new_ss.GetTree(string(scid)); err != nil {
			return
		}
		if write_tree, err = write_ss.GetTree(string(scid)); err != nil {
			return
		}
		c := new_tree.Cursor()
		for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
			write_tree.Put(k, v)
		}
		sc_trees = append(sc_trees, write_tree)
	}

	for _, scid := range sc_list_modified {
		if old_tree, err = old_ss.GetTree(string(scid)); err != nil {
			return
		}
		if new_tree, err = new_ss.GetTree(string(scid)); err != nil {
			return
		}
		if write_tree, err = write_ss.GetTree(string(scid)); err != nil {
			return
		}

		clone_tree_changes(old_tree, new_tree, write_tree)

		sc_trees = append(sc_trees, write_tree)
	}

	latest_commit_version, err = graviton.Commit(sc_trees...)
	return
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
		var latest_commit_version uint64
		latest_commit_version, err = clone_snapshot(store.Balance_store, write_store, toporecord.State_Version)
		major_copy = latest_commit_version
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

			// fetch old tree data
			old_topo := i
			new_topo := i + 1

			err = nil
			if old_toporecord, err = store.Topo_store.Read(old_topo); err == nil {

				if new_toporecord, err = store.Topo_store.Read(new_topo); err == nil {
					var latest_commit_version uint64
					latest_commit_version, err = diff_snapshot(store.Balance_store, write_store, old_toporecord.State_Version, new_toporecord.State_Version)
					if err != nil {
						return err
					}

					new_entries = append(new_entries, new_topo)
					commit_versions = append(commit_versions, latest_commit_version)

				}
			}
			if err != nil {
				return
			}

			if i%1000 == 0 {
				globals.Logger.Info("Commiting block to block changes", "done", float64(i*100)/float64(max_topoheight))
			}
		}

		// now lets store all the commit versions in 1 go
		for i, topo := range new_entries {
			old_toporecord, err := store.Topo_store.Read(topo)
			if err != nil {
				globals.Logger.Error(err, "err reading/writing toporecord", "topo", topo)
				return err
			}

			store.Topo_store.Write(topo, old_toporecord.BLOCK_ID, commit_versions[i], old_toporecord.Height)

			/*{
				ss, _ := write_store.LoadSnapshot(commit_versions[i])
				balance_tree, _ := ss.GetTree(config.BALANCE_TREE)
				sc_meta_tree, _ := ss.GetTree(config.SC_META)
				balance_merkle_hash, _ := balance_tree.Hash()
				meta_merkle_hash, _ := sc_meta_tree.Hash()

				var hash [32]byte
				for i := range balance_merkle_hash {
					hash[i] = balance_merkle_hash[i] ^ meta_merkle_hash[i]
				}
				fmt.Printf("writing toporecord %d version %d hash %x\n", topo, commit_versions[i], hash[:])
			}*/
			var bl block.Block
			var block_data []byte
			if block_data, err = store.Block_tx_store.ReadBlock(old_toporecord.BLOCK_ID); err != nil {
				return err
			}
			if err = bl.Deserialize(block_data); err != nil { // we should deserialize the block here
				return err
			}

			var diff *big.Int
			if diff, err = store.Block_tx_store.ReadBlockDifficulty(old_toporecord.BLOCK_ID); err != nil {
				return err
			}

			store.Block_tx_store.DeleteBlock(old_toporecord.BLOCK_ID)
			err = store.Block_tx_store.WriteBlock(old_toporecord.BLOCK_ID, block_data, diff, commit_versions[i], bl.Height)
			if err != nil {
				return err
			}

			if i%1000 == 0 {
				globals.Logger.Info("Rewriting entries", "done", float64(i*100)/float64(len(new_entries)))
			}

		}
	}

	// now overwrite the starting topo mapping
	for i := int64(0); i <= prune_topoheight; i++ { // overwrite the entries in the topomap
		if toporecord, err := store.Topo_store.Read(i); err == nil {
			store.Topo_store.Write(i, toporecord.BLOCK_ID, major_copy, toporecord.Height)
			//fmt.Printf("writing toporecord %d version %d\n",i, major_copy)
		} else {
			globals.Logger.Error(err, "err reading toporecord", "topo", i)
			return err // this is irrepairable  damage
		}
		if i%1000 == 0 {
			globals.Logger.Info("Filling gaps", "done", float64(i*100)/float64(prune_topoheight))
		}
	}

	// now lets remove the old graviton db
	write_store.Close()

	return

}

// clone tree changes between 2 versions (old_tree, new_tree and then commit them to write_tree)
func clone_tree_changes(old_tree, new_tree, write_tree *graviton.Tree) {
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

	delete_handler := func(k, v []byte) { // modification receives old value
		write_tree.Delete(k)
	}

	graviton.Diff(old_tree, new_tree, delete_handler, modify_handler, insert_handler)
}
