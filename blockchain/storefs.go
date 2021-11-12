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

type storefs struct {
	basedir string
}

// the filename stores the following information
// hex  block id (64 chars).block._ rewards (decimal) _ difficulty _ cumulative difficulty

func (s *storefs) ReadBlock(h [32]byte) ([]byte, error) {
	var dummy [32]byte
	if h == dummy {
		return nil, fmt.Errorf("empty block")
	}

	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {
			//fmt.Printf("Reading block with filename %s\n", file.Name())
			file := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]), file.Name())
			return ioutil.ReadFile(file)
		}
	}

	return nil, os.ErrNotExist
}

func (s *storefs) DeleteBlock(h [32]byte) error {
	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {
			file := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]), file.Name())
			return os.Remove(file)
		}
	}

	return os.ErrNotExist
}

func (s *storefs) ReadBlockDifficulty(h [32]byte) (*big.Int, error) {
	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {

			diff := new(big.Int)

			parts := strings.Split(file.Name(), "_")
			if len(parts) != 3 {
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

func (chain *Blockchain) ReadBlockSnapshotVersion(h [32]byte) (uint64, error) {
	return chain.Store.Block_tx_store.ReadBlockSnapshotVersion(h)
}
func (s *storefs) ReadBlockSnapshotVersion(h [32]byte) (uint64, error) {
	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	filename_start := fmt.Sprintf("%x.block", h[:])
	for _, file := range files {
		if strings.HasPrefix(file.Name(), filename_start) {

			var diff uint64

			parts := strings.Split(file.Name(), "_")
			if len(parts) != 3 {
				panic("such filename cannot occur")
			}

			_, err := fmt.Sscan(parts[2], &diff)
			if err != nil {
				return 0, err
			}
			return diff, nil
		}
	}

	return 0, os.ErrNotExist
}

func (s *storefs) WriteBlock(h [32]byte, data []byte, difficulty *big.Int, ss_version uint64) (err error) {
	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))
	file := filepath.Join(dir, fmt.Sprintf("%x.block_%s_%d", h[:], difficulty.String(), ss_version))
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(file, data, 0600)
}

func (s *storefs) ReadTX(h [32]byte) ([]byte, error) {
	file := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]), fmt.Sprintf("%x.tx", h[:]))
	return ioutil.ReadFile(file)
}

func (s *storefs) WriteTX(h [32]byte, data []byte) (err error) {
	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))
	file := filepath.Join(dir, fmt.Sprintf("%x.tx", h[:]))

	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return ioutil.WriteFile(file, data, 0600)
}

func (s *storefs) DeleteTX(h [32]byte) (err error) {
	dir := filepath.Join(filepath.Join(s.basedir, "bltx_store"), fmt.Sprintf("%02x", h[0]), fmt.Sprintf("%02x", h[1]), fmt.Sprintf("%02x", h[2]))
	file := filepath.Join(dir, fmt.Sprintf("%x.tx", h[:]))
	return os.Remove(file)
}
