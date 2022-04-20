package dvm

import "time"
import "testing"
import "math/rand"

func TestSC_META_DATA(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 256; i++ {
		sctype := byte(i)
		var meta, meta2 SC_META_DATA
		for j := 0; j < 10000; j++ {
			meta.Type = sctype
			rand.Read(meta.DataHash[:])

			ser := meta.MarshalBinaryGood()
			if err := meta2.UnmarshalBinaryGood(ser[:]); err != nil {
				t.Fatalf("marshallling unmarshalling failed")
			}
			if meta != meta2 {
				t.Fatalf("marshallling unmarshalling failed")
			}
		}
	}
}
