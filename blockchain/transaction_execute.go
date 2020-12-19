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

// this file implements  core execution of all changes to block chain homomorphically

import "math/big"
import "golang.org/x/xerrors"

import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/graviton"

// process the miner tx, giving fees, miner rewatd etc
func (chain *Blockchain) process_miner_transaction(tx transaction.Transaction, genesis bool, balance_tree *graviton.Tree, fees uint64) {

	var acckey crypto.Point
	if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
		panic(err)
	}

	if genesis == true { // process premine ,register genesis block, dev key
		balance := crypto.ConstructElGamal(acckey.G1(), crypto.ElGamal_BASE_G) // init zero balance
		balance = balance.Plus(new(big.Int).SetUint64(tx.Value))               // add premine to users balance homomorphically
		balance_tree.Put(tx.MinerAddress[:], balance.Serialize())              // reserialize and store
		return
	}

	// general coin base transaction
	balance_serialized, err := balance_tree.Get(tx.MinerAddress[:])
	if err != nil {
		panic(err)
	}

	balance := new(crypto.ElGamal).Deserialize(balance_serialized)
	balance = balance.Plus(new(big.Int).SetUint64(fees + 50000)) // add fees colllected to users balance homomorphically
	balance_tree.Put(tx.MinerAddress[:], balance.Serialize())    // reserialize and store
	return

}

// process the tx, giving fees, miner rewatd etc
// this should be atomic, either all should be done or none at all
func (chain *Blockchain) process_transaction(tx transaction.Transaction, balance_tree *graviton.Tree) uint64 {

	//fmt.Printf("Processing/Executing transaction %s %s\n", tx.GetHash(), tx.TransactionType.String())
	switch tx.TransactionType {

	case transaction.REGISTRATION:
		if _, err := balance_tree.Get(tx.MinerAddress[:]); err != nil {
			if !xerrors.Is(err, graviton.ErrNotFound) { // any other err except not found panic
				panic(err)
			}
		} // address needs registration

		var acckey crypto.Point
		if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
			panic(err)
		}

		zerobalance := crypto.ConstructElGamal(acckey.G1(), crypto.ElGamal_BASE_G)
		zerobalance = zerobalance.Plus(new(big.Int).SetUint64(800000)) // add fix amount to every wallet to users balance for more testing

		balance_tree.Put(tx.MinerAddress[:], zerobalance.Serialize())

		return 0 // registration doesn't give any fees . why & how ?

	case transaction.NORMAL:
		for i := range tx.Statement.Publickeylist_compressed {
			if balance_serialized, err := balance_tree.Get(tx.Statement.Publickeylist_compressed[i][:]); err == nil {

				balance := new(crypto.ElGamal).Deserialize(balance_serialized)
				echanges := crypto.ConstructElGamal(tx.Statement.C[i], tx.Statement.D)

				balance = balance.Add(echanges)                                                    // homomorphic addition of changes
				balance_tree.Put(tx.Statement.Publickeylist_compressed[i][:], balance.Serialize()) // reserialize and store
			} else {
				panic(err) // if balance could not be obtained panic ( we can never reach here, otherwise how tx got verified)
			}

		}
		return tx.Statement.Fees

	default:
		panic("unknown transaction, do not know how to process it")
		return 0
	}

}
