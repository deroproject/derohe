package astrobwtv3

import (
	"encoding/hex"
	"testing"

	sha256 "github.com/minio/sha256-simd"
)

// TestSha256SimdNistVectors confirms the vendored minio/sha256-simd v1.0.1
// produces the exact SHA-256 output specified by NIST FIPS 180-4 for a set
// of known vectors. This is the byte-level correctness gate for the
// consensus-critical sha256.Sum256 calls in pow.go (AstroBWTv3 PoW).
func TestSha256SimdNistVectors(t *testing.T) {
	vectors := []struct {
		input  string
		expect string // hex digest
	}{
		// "abc" — the canonical NIST FIPS 180-4 / RFC 6234 vector.
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		// empty input.
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		// 56-byte input.
		{"abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
			"248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},
		// 112-byte (two-block) — FIPS 180-4 Appendix B example.
		{"abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrstu",
			"cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
	}

	for _, v := range vectors {
		got := sha256.Sum256([]byte(v.input))
		gotHex := hex.EncodeToString(got[:])
		if gotHex != v.expect {
			t.Fatalf("sha256.Sum256(%q) = %s, want %s", v.input, gotHex, v.expect)
		}
	}
}
