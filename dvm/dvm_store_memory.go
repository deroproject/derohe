// Copyright 2017-2018 DERO Project. All rights reserved.
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

package dvm

import "fmt"

// this file implements a RAM store backend for testing purposes

type Memory_Storage struct {
	Atoms   []DataAtom // all modification operations have to played/reverse in this order
	Keys    map[DataKey]Variable
	RawKeys map[string][]byte
}

var Memory_Backend Memory_Storage

func init() {
	DVM_STORAGE_BACKEND = &Memory_Backend
}

func (mem_store *Memory_Storage) RawLoad(key []byte) (value []byte, found bool) {
	value, found = mem_store.RawKeys[string(key)]
	return
}

func (mem_store *Memory_Storage) RawStore(key []byte, value []byte) {
	mem_store.RawKeys[string(key)] = value
	return
}

// this will load the variable, and if the key is found
func (mem_store *Memory_Storage) Load(dkey DataKey, found_value *uint64) (value Variable) {

	*found_value = 0
	// if it was modified in current TX, use it
	if result, ok := mem_store.Keys[dkey]; ok {
		*found_value = 1
		return result
	}

	return
}

// store variable
func (mem_store *Memory_Storage) Store(dkey DataKey, v Variable) {

	fmt.Printf("Storing %+v   : %+v\n", dkey, v)
	var found uint64
	old_value := mem_store.Load(dkey, &found)

	var atom DataAtom
	atom.Key = dkey
	atom.Value = v
	if found != 0 {
		atom.Prev_Value = old_value
	} else {
		atom.Prev_Value = Variable{}
	}

	mem_store.Keys[atom.Key] = atom.Value
	mem_store.Atoms = append(mem_store.Atoms, atom)

}
