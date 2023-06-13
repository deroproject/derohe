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

package crypto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/graviton"
)

type Statement struct {
	RingSize                 uint64
	CLn                      []*bn256.G1
	CRn                      []*bn256.G1
	Bytes_per_publickey      byte        // number of bytes need per public key, it will be from 1 to 32 bytes long, but will rarely be more than 4 byte
	Publickeylist_pointers   []byte      // all the public keys are hashed and there necessary bits taken from the start to reach graviton leaf
	Publickeylist            []*bn256.G1 // Todo these can be skipped and collected back later on from the chain, this will save ringsize * POINTSIZE bytes
	Publickeylist_compressed [][33]byte  // compressed format for public keys NOTE: only valid in deserialized transactions
	C                        []*bn256.G1 // commitments
	D                        *bn256.G1
	Fees                     uint64

	Roothash [32]byte // note roothash contains the merkle root hash of chain, when it was build
}

type Witness struct {
	SecretKey      *big.Int
	R              *big.Int
	TransferAmount uint64 // total value being transferred
	Balance        uint64 // whatever is the the amount left after transfer
	Index          []int  // index of sender in the public key list

}

func (s *Statement) Serialize(w *bytes.Buffer) {
	buf := make([]byte, binary.MaxVarintLen64)
	//n := binary.PutUvarint(buf, uint64(len(s.Publickeylist)))
	//w.Write(buf[:n])

	if len(s.Publickeylist_pointers) == 0 {
		power := byte(GetPowerof2(len(s.Publickeylist))) // len(s.Publickeylist) is always power of 2
		w.WriteByte(power)
		w.WriteByte(s.Bytes_per_publickey)

		n := binary.PutUvarint(buf, s.Fees)
		w.Write(buf[:n])

		w.Write(s.D.EncodeCompressed())
		s.Publickeylist_pointers = s.Publickeylist_pointers[:0]
		for i := 0; i < len(s.Publickeylist); i++ {
			hashed_key := graviton.Sum(s.Publickeylist[i].EncodeCompressed())
			w.Write(hashed_key[:s.Bytes_per_publickey])
			s.Publickeylist_pointers = append(s.Publickeylist_pointers, hashed_key[:s.Bytes_per_publickey]...)
		}
	} else {
		power := byte(GetPowerof2(len(s.Publickeylist_pointers) / int(s.Bytes_per_publickey))) // len(s.Publickeylist) is always power of 2
		w.WriteByte(power)
		w.WriteByte(s.Bytes_per_publickey)
		n := binary.PutUvarint(buf, s.Fees)
		w.Write(buf[:n])
		w.Write(s.D.EncodeCompressed())
		w.Write(s.Publickeylist_pointers[:])
	}

	for i := 0; i < len(s.Publickeylist_pointers)/int(s.Bytes_per_publickey); i++ {
		//     w.Write( s.CLn[i].EncodeCompressed()) /// this is expanded from graviton store
		//     w.Write( s.CRn[i].EncodeCompressed())  /// this is expanded from graviton store
		//w.Write(s.Publickeylist[i].EncodeCompressed())  /// this is expanded from graviton store
		w.Write(s.C[i].EncodeCompressed())
	}

	w.Write(s.Roothash[:])

}

func (s *Statement) Deserialize(r *bytes.Reader) error {

	var err error
	//var buf [32]byte
	var bufp [33]byte

	length, err := r.ReadByte()
	if err != nil {
		return err
	}
	s.RingSize = 1 << length
	if s.RingSize > 128 {
		return fmt.Errorf("ring size is too large")
	}

	s.Bytes_per_publickey, err = r.ReadByte()
	if err != nil {
		return err
	}

	s.Fees, err = binary.ReadUvarint(r)
	if err != nil {
		return err
	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		s.D = &p
	} else {
		return err
	}

	s.CLn = s.CLn[:0]
	s.CRn = s.CRn[:0]
	s.Publickeylist = s.Publickeylist[:0]
	s.Publickeylist_compressed = s.Publickeylist_compressed[:0]
	s.Publickeylist_pointers = s.Publickeylist_pointers[:0]
	s.C = s.C[:0]

	s.Publickeylist_pointers = make([]byte, s.RingSize*uint64(s.Bytes_per_publickey), s.RingSize*uint64(s.Bytes_per_publickey))

	// read all compressed pointers in 1 go
	if n, err := r.Read(s.Publickeylist_pointers); n == int(s.RingSize*uint64(s.Bytes_per_publickey)) && err == nil {

	} else {
		return err
	}

	for i := uint64(0); i < s.RingSize; i++ {

		/*
		       if n,err := r.Read(bufp[:]); n == 33 && err == nil {
		           var p bn256.G1
		           if  err =  p.DecodeCompressed(bufp[:]); err != nil {
		           return err
		           }
		           s.CLn = append(s.CLn,&p)
		   }else{
		       return err
		   }

		       if n,err := r.Read(bufp[:]); n == 33 && err == nil {
		           var p bn256.G1
		           if  err =  p.DecodeCompressed(bufp[:]); err != nil {
		           return err
		           }
		           s.CRn = append(s.CRn,&p)
		   }else{
		       return err
		   }
		*/

		/*
			if n, err := r.Read(bufp[:]); n == 33 && err == nil {
				var p bn256.G1
				var pcopy [33]byte
				copy(pcopy[:], bufp[:])
				if err = p.DecodeCompressed(bufp[:]); err != nil {
					return err
				}
				s.Publickeylist_compressed = append(s.Publickeylist_compressed, pcopy)
				s.Publickeylist = append(s.Publickeylist, &p)
			} else {
				return err
			}
		*/

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			s.C = append(s.C, &p)
		} else {
			return err
		}

	}

	if n, err := r.Read(s.Roothash[:]); n == 32 && err == nil {

	} else {
		return err
	}

	return nil

}

/*
type Proof struct {
	BA *bn256.G1
	BS *bn256.G1
	A  *bn256.G1
	B  *bn256.G1

	CLnG, CRnG, C_0G, DG, y_0G, gG, C_XG, y_XG []*bn256.G1

	u *bn256.G1

	f *FieldVector

	z_A *big.Int

	T_1  *bn256.G1
	T_2  *bn256.G1

	that *big.Int
	mu   *big.Int

	c                     *big.Int
	s_sk, s_r, s_b, s_tau *big.Int

	//ip *InnerProduct
}
*/

