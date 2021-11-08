// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bn256

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestG1Array(t *testing.T) {
	count := 8

	var g1array G1Array
	var g1array_opt G1Array

	for i := 0; i < count; i++ {
		a, _ := rand.Int(rand.Reader, Order)
		g1array = append(g1array, new(G1).ScalarBaseMult(a))
		g1array_opt = append(g1array_opt, new(G1).ScalarBaseMult(a))
	}
	g1array_opt.MakeAffine()
	for i := range g1array_opt {
		require.Equal(t, g1array_opt[i].p.z, *newGFp(1)) // current we are not testing points of infinity
	}
}

func benchmarksingleinverts(count int, b *testing.B) {
	var g1array, g1backup G1Array

	for i := 0; i < count; i++ {
		a, _ := rand.Int(rand.Reader, Order)
		g1backup = append(g1backup, new(G1).ScalarBaseMult(a))
	}

	for n := 0; n < b.N; n++ {
		g1array = g1array[:0]
		for i := range g1backup {
			g1array = append(g1array, new(G1).Set(g1backup[i]))
			g1array[i].p.MakeAffine()
		}
	}
}

func benchmarkbatchedinverts(count int, b *testing.B) {
	var g1array, g1backup G1Array

	for i := 0; i < count; i++ {
		a, _ := rand.Int(rand.Reader, Order)
		g1backup = append(g1backup, new(G1).ScalarBaseMult(a))
	}

	for n := 0; n < b.N; n++ {
		g1array = g1array[:0]
		for i := range g1backup {
			g1array = append(g1array, new(G1).Set(g1backup[i]))
		}
		g1array.MakeAffine()
	}
}

func BenchmarkInverts_Single_256(b *testing.B)  { benchmarksingleinverts(256, b) }
func BenchmarkInverts_Batched_256(b *testing.B) { benchmarkbatchedinverts(256, b) }
