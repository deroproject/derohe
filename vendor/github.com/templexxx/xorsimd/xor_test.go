// Copyright (c) 2019. Temple3x (temple3x@gmail.com)
//
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.
//
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// TestEncodeBytes is copied from Go Standard lib:
// crypto/cipher/xor_test.go

package xorsimd

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"
	"unsafe"
)

const (
	kb = 1024
	mb = 1024 * 1024

	testSize = kb
)

func TestBytes8(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 0; j < 1024; j++ {
		a := make([]byte, 8)
		b := make([]byte, 8)
		fillRandom(a)
		fillRandom(b)

		dst0 := make([]byte, 8)
		Bytes8(dst0, a, b)

		dst1 := make([]byte, 8)
		for i := 0; i < 8; i++ {
			dst1[i] = a[i] ^ b[i]
		}

		if !bytes.Equal(dst0, dst1) {
			t.Fatal("not equal", a, b, dst0, dst1)
		}
	}
}

func TestBytes16(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 0; j < 1024; j++ {
		a := make([]byte, 16)
		b := make([]byte, 16)
		fillRandom(a)
		fillRandom(b)

		dst0 := make([]byte, 16)
		Bytes16(dst0, a, b)

		dst1 := make([]byte, 16)
		for i := 0; i < 16; i++ {
			dst1[i] = a[i] ^ b[i]
		}

		if !bytes.Equal(dst0, dst1) {
			t.Fatal("not equal", dst0, dst1, a, b)
		}
	}
}

const wordSize = int(unsafe.Sizeof(uintptr(0)))

func TestBytes8Align(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 0; j < 1024; j++ {
		a := make([]byte, 8+wordSize)
		b := make([]byte, 8+wordSize)
		dst0 := make([]byte, 8+wordSize)
		dst1 := make([]byte, 8+wordSize)

		al := alignment(a)
		offset := 0
		if al != 0 {
			offset = wordSize - al
		}
		a = a[offset : offset+8]

		al = alignment(b)
		offset = 0
		if al != 0 {
			offset = wordSize - al
		}
		b = b[offset : offset+8]

		al = alignment(dst0)
		offset = 0
		if al != 0 {
			offset = wordSize - al
		}
		dst0 = dst0[offset : offset+8]

		al = alignment(dst1)
		offset = 0
		if al != 0 {
			offset = wordSize - al
		}
		dst1 = dst1[offset : offset+8]

		fillRandom(a)
		fillRandom(b)

		Bytes8Align(dst0, a, b)

		for i := 0; i < 8; i++ {
			dst1[i] = a[i] ^ b[i]
		}

		if !bytes.Equal(dst0, dst1) {
			t.Fatal("not equal", a, b, dst0, dst1)
		}
	}
}

func alignment(s []byte) int {
	return int(uintptr(unsafe.Pointer(&s[0])) & uintptr(wordSize-1))
}

func TestBytes16Align(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 0; j < 1024; j++ {
		a := make([]byte, 16+wordSize)
		b := make([]byte, 16+wordSize)
		dst0 := make([]byte, 16+wordSize)
		dst1 := make([]byte, 16+wordSize)

		al := alignment(a)
		offset := 0
		if al != 0 {
			offset = wordSize - al
		}
		a = a[offset : offset+16]

		al = alignment(b)
		offset = 0
		if al != 0 {
			offset = wordSize - al
		}
		b = b[offset : offset+16]

		al = alignment(dst0)
		offset = 0
		if al != 0 {
			offset = wordSize - al
		}
		dst0 = dst0[offset : offset+16]

		al = alignment(dst1)
		offset = 0
		if al != 0 {
			offset = wordSize - al
		}
		dst1 = dst1[offset : offset+16]

		fillRandom(a)
		fillRandom(b)

		Bytes16Align(dst0, a, b)

		for i := 0; i < 16; i++ {
			dst1[i] = a[i] ^ b[i]
		}

		if !bytes.Equal(dst0, dst1) {
			t.Fatal("not equal", a, b, dst0, dst1)
		}
	}
}

func TestBytesA(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 2; j <= 1024; j++ {

		for alignP := 0; alignP < 2; alignP++ {
			p := make([]byte, j)[alignP:]
			q := make([]byte, j)
			d1 := make([]byte, j)
			d2 := make([]byte, j)

			fillRandom(p)
			fillRandom(q)

			BytesA(d1, p, q)
			for i := 0; i < j-alignP; i++ {
				d2[i] = p[i] ^ q[i]
			}
			if !bytes.Equal(d1, d2) {
				t.Fatal("not equal")
			}
		}
	}
}

func TestBytesB(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 2; j <= 1024; j++ {

		for alignQ := 0; alignQ < 2; alignQ++ {
			p := make([]byte, j)
			q := make([]byte, j)[alignQ:]
			d1 := make([]byte, j)
			d2 := make([]byte, j)

			fillRandom(p)
			fillRandom(q)

			BytesB(d1, p, q)
			for i := 0; i < j-alignQ; i++ {
				d2[i] = p[i] ^ q[i]
			}
			if !bytes.Equal(d1, d2) {
				t.Fatal("not equal")
			}
		}
	}
}

func TestBytes(t *testing.T) {

	rand.Seed(time.Now().UnixNano())

	for j := 1; j <= 1024; j++ {

		for alignP := 0; alignP < 2; alignP++ {
			for alignQ := 0; alignQ < 2; alignQ++ {
				for alignD := 0; alignD < 2; alignD++ {
					p := make([]byte, j)[alignP:]
					q := make([]byte, j)[alignQ:]
					d1 := make([]byte, j)[alignD:]
					d2 := make([]byte, j)[alignD:]

					fillRandom(p)
					fillRandom(q)

					Bytes(d1, p, q)
					n := min(p, q, d1)
					for i := 0; i < n; i++ {
						d2[i] = p[i] ^ q[i]
					}
					if !bytes.Equal(d1, d2) {
						t.Fatal("not equal")
					}
				}
			}
		}
	}
}

func min(a, b, c []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if len(c) < n {
		n = len(c)
	}
	return n
}

func TestEncodeWithFeature(t *testing.T) {
	max := testSize

	switch getCPUFeature() {
	case avx512:
		testEncode(t, max, sse2, -1)
		testEncode(t, max, avx2, sse2)
		testEncode(t, max, avx512, avx2)
	case avx2:
		testEncode(t, max, sse2, -1)
		testEncode(t, max, avx2, sse2)
	case sse2:
		testEncode(t, max, sse2, -1)
	case generic:
		testEncode(t, max, generic, -1)
	}
}

func testEncode(t *testing.T, maxSize, feat, cmpFeat int) {

	rand.Seed(time.Now().UnixNano())
	srcN := randIntn(10, 2) // Cannot be 1, see func encode(dst []byte, src [][]byte, feature int).

	fs := featToStr(feat)
	for size := 1; size <= maxSize; size++ {
		exp := make([]byte, size)
		src := make([][]byte, srcN)
		for j := 0; j < srcN; j++ {
			src[j] = make([]byte, size)
			fillRandom(src[j])
		}

		if cmpFeat < 0 {
			encodeTested(exp, src)
		} else {
			cpuFeature = cmpFeat
			Encode(exp, src)
		}

		act := make([]byte, size)
		cpuFeature = feat
		Encode(act, src)

		if !bytes.Equal(exp, act) {
			t.Fatalf("%s mismatched with %s, src_num: %d, size: %d",
				fs, featToStr(cmpFeat), srcN, size)
		}
	}

	t.Logf("%s pass src_num:%d, max_size: %d",
		fs, srcN, maxSize)
}

func featToStr(f int) string {
	switch f {
	case avx512:
		return "AVX512"
	case avx2:
		return "AVX2"
	case sse2:
		return "SSE2"
	case generic:
		return "Generic"
	default:
		return "Tested"
	}
}

func encodeTested(dst []byte, src [][]byte) {

	n := len(dst)
	for i := 0; i < n; i++ {
		s := src[0][i]
		for j := 1; j < len(src); j++ {
			s ^= src[j][i]
		}
		dst[i] = s
	}
}

// randIntn returns, as an int, a non-negative pseudo-random number in [min,n)
// from the default Source.
func randIntn(n, min int) int {
	m := rand.Intn(n)
	if m < min {
		m = min
	}
	return m
}

func BenchmarkBytes8(b *testing.B) {
	s0 := make([]byte, 8)
	s1 := make([]byte, 8)
	fillRandom(s0)
	fillRandom(s1)
	dst0 := make([]byte, 8)

	b.ResetTimer()
	b.SetBytes(8)
	for i := 0; i < b.N; i++ {
		Bytes8(dst0, s0, s1)
	}
}

func BenchmarkBytes16(b *testing.B) {
	s0 := make([]byte, 16)
	s1 := make([]byte, 16)
	fillRandom(s0)
	fillRandom(s1)
	dst0 := make([]byte, 16)

	b.ResetTimer()
	b.SetBytes(16)
	for i := 0; i < b.N; i++ {
		Bytes16(dst0, s0, s1)
	}
}

func BenchmarkBytesN_16Bytes(b *testing.B) {
	s0 := make([]byte, 16)
	s1 := make([]byte, 16)
	fillRandom(s0)
	fillRandom(s1)
	dst0 := make([]byte, 16)

	b.ResetTimer()
	b.SetBytes(16)
	for i := 0; i < b.N; i++ {
		BytesA(dst0, s0, s1)
	}
}

func BenchmarkEncode(b *testing.B) {
	sizes := []int{4 * kb, mb, 8 * mb}

	srcNums := []int{5, 10}

	var feats []int
	switch getCPUFeature() {
	case avx512:
		feats = append(feats, avx512)
		feats = append(feats, avx2)
		feats = append(feats, sse2)
	case avx2:
		feats = append(feats, avx2)
		feats = append(feats, sse2)
	case sse2:
		feats = append(feats, sse2)
	default:
		feats = append(feats, generic)
	}

	b.Run("", benchEncRun(benchEnc, srcNums, sizes, feats))
}

func benchEncRun(f func(*testing.B, int, int, int), srcNums, sizes, feats []int) func(*testing.B) {
	return func(b *testing.B) {
		for _, feat := range feats {
			for _, srcNum := range srcNums {
				for _, size := range sizes {
					b.Run(fmt.Sprintf("(%d+1)-%s-%s", srcNum, byteToStr(size), featToStr(feat)), func(b *testing.B) {
						f(b, srcNum, size, feat)
					})
				}
			}
		}
	}
}

func benchEnc(b *testing.B, srcNum, size, feat int) {
	dst := make([]byte, size)
	src := make([][]byte, srcNum)
	for i := 0; i < srcNum; i++ {
		src[i] = make([]byte, size)
		fillRandom(src[i])
	}
	cpuFeature = feat

	b.SetBytes(int64((srcNum + 1) * size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encode(dst, src)
	}
}

func fillRandom(p []byte) {
	rand.Read(p)
}

func byteToStr(n int) string {
	if n >= mb {
		return fmt.Sprintf("%dMB", n/mb)
	}

	return fmt.Sprintf("%dKB", n/kb)
}
