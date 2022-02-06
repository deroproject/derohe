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

// this file implements a filesystem store which is used to store blocks/transactions directly in the file system

import "os"
import "fmt"
import "strings"
import "io/ioutil"
import "math/big"
import "path/filepath"
import "github.com/deroproject/derohe/globals"

type storefs struct {
	basedir string
}

// TODO we need to enable big support or shift to object store at some point in time
func (s *storefs) getpath(h [32]byte) string {
	// if you wish to use 3 level indirection, it will cause 16 million inodes to be used, but system will be faster
	//return  filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))
	// currently we are settling on 65536 inodes
	return filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]))
}

// the filename stores the following information
// hex  block id (64 chars).block._ rewards (decimal) _ difficulty _ cumulative difficulty

func (s *storefs) ReadBlock(h [32]byte) ([]byte, error) {
	defer globals.Recover(0)
	var dummy [32]byte
	if h == dummy {
		return nil, fmt.Errorf("empty block")
	}

	dir := s.getpath(h)

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {
			//fmt.Printf("Reading block with filename %s\n", file.Name())
			file := filepath.Join(dir, file.Name())
			return os.ReadFile(file)
		}
	}

	return nil, os.ErrNotExist
}

// on windows, we see an odd behaviour where some files could not be deleted, since they may exist only in cache
func (s *storefs) DeleteBlock(h [32]byte) error {
	dir := s.getpath(h)

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	var found bool
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {
			file := filepath.Join(dir, file.Name())
			err = os.Remove(file)
			if err != nil {
				//return err
			}
			found = true
		}
	}
	if found {
		return nil
	}

	return os.ErrNotExist
}

func (s *storefs) ReadBlockDifficulty(h [32]byte) (*big.Int, error) {
	dir := s.getpath(h)

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {

			diff := new(big.Int)

			parts := strings.Split(file.Name(), "_")
			if len(parts) != 4 {
				panic("such filename cannot occur")
			}

			_, err := fmt.Sscan(parts[1], diff)
			if err != nil {
				return nil, err
			}
			return diff, nil
		}
	}

	return nil, os.ErrNotExist
}

// this cannot be cached
func (chain *Blockchain) ReadBlockSnapshotVersion(h [32]byte) (uint64, error) {
	return chain.Store.Block_tx_store.ReadBlockSnapshotVersion(h)
}
func (s *storefs) ReadBlockSnapshotVersion(h [32]byte) (uint64, error) {
	dir := s.getpath(h)

	files, err := os.ReadDir(dir) // this always returns the sorted list
	if err != nil {
		return 0, err
	}
	// windows has a caching issue, so earlier versions may exist at the same time
	// so we mitigate it, by using the last version, below 3 lines reverse the already sorted arrray
	for left, right := 0, len(files)-1; left < right; left, right = left+1, right-1 {
		files[left], files[right] = files[right], files[left]
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {
			var ssversion uint64
			parts := strings.Split(file.Name(), "_")
			if len(parts) != 4 {
				panic("such filename cannot occur")
			}
			_, err := fmt.Sscan(parts[2], &ssversion)
			if err != nil {
				return 0, err
			}
			return ssversion, nil
		}
	}

	return 0, os.ErrNotExist
}

func (chain *Blockchain) ReadBlockHeight(h [32]byte) (uint64, error) {
	if heighti, ok := chain.cache_BlockHeight.Get(h); ok {
		height := heighti.(uint64)
		return height, nil
	}

	height, err := chain.Store.Block_tx_store.ReadBlockHeight(h)
	if err == nil && chain.cache_enabled {
		chain.cache_BlockHeight.Add(h, height)
	}
	return height, err
}

func (s *storefs) ReadBlockHeight(h [32]byte) (uint64, error) {
	dir := s.getpath(h)

	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {
			var height uint64
			parts := strings.Split(file.Name(), "_")
			if len(parts) != 4 {
				panic("such filename cannot occur")
			}
			_, err := fmt.Sscan(parts[3], &height)
			if err != nil {
				return 0, err
			}
			return height, nil
		}
	}

	return 0, os.ErrNotExist
}

func (s *storefs) WriteBlock(h [32]byte, data []byte, difficulty *big.Int, ss_version uint64, height uint64) (err error) {
	dir := s.getpath(h)
	file := filepath.Join(dir, fmt.Sprintf("%x.block_%s_%d_%d", h[:], difficulty.String(), ss_version, height))
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(file, data, 0600)
}

func (s *storefs) ReadTX(h [32]byte) ([]byte, error) {
	dir := s.getpath(h)
	file := filepath.Join(dir, fmt.Sprintf("%x.tx", h[:]))
	return ioutil.ReadFile(file)
}

func (s *storefs) WriteTX(h [32]byte, data []byte) (err error) {
	dir := s.getpath(h)
	file := filepath.Join(dir, fmt.Sprintf("%x.tx", h[:]))

	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return ioutil.WriteFile(file, data, 0600)
}

func (s *storefs) DeleteTX(h [32]byte) (err error) {
	dir := s.getpath(h)
	file := filepath.Join(dir, fmt.Sprintf("%x.tx", h[:]))
	return os.Remove(file)
}
