package bn256

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertGFpEqual(t *testing.T, a, b *gfP) {
	for i := 0; i < 4; i++ {
		assert.Equal(t, a[i], b[i], fmt.Sprintf("The %d's elements differ between the 2 field elements", i))
	}
}

func TestEncodeCompressed(t *testing.T) {
	// Case1: Create random point (Jacobian form)
	_, GaInit, err := RandomG1(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Affine form of GaInit
	GaAffine := new(G1)
	GaAffine.Set(GaInit)
	GaAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeCompress function
	GaCopy1 := new(G1)
	GaCopy1.Set(GaInit)
	compressed := GaCopy1.EncodeCompressed()

	// Encode GaCopy2 with the Marshal function
	GaCopy2 := new(G1)
	GaCopy2.Set(GaInit)
	marshalled := GaCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		compressed[1:],  // Ignore the masking byte
		marshalled[:32], // Get only the x-coordinate
		"The EncodeCompressed and Marshal function yield different results for the x-coordinate")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 := new(G1)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeCompress function
	Gb2 := new(G1)
	err = Gb2.DecodeCompressed(compressed)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decompressed point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decompressed point should equal the y-coord of the intial point")

	// Case2: Encode the point at infinity
	GInfinity := new(G1)
	GInfinity.p = &curvePoint{}
	GInfinity.p.SetInfinity()

	// Get the point in affine form
	GInfinityAffine := new(G1)
	GInfinityAffine.Set(GInfinity)
	GInfinityAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeCompress function
	GInfinityCopy1 := new(G1)
	GInfinityCopy1.Set(GInfinity)
	compressed = GInfinityCopy1.EncodeCompressed()

	// Encode GaCopy2 with the Marshal function
	GInfinityCopy2 := new(G1)
	GInfinityCopy2.Set(GInfinity)
	marshalled = GInfinityCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		compressed[1:], // Ignore the masking byte
		marshalled[:32],
		"The EncodeCompressed and Marshal function yield different results")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 = new(G1)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeCompress function
	Gb2 = new(G1)
	err = Gb2.DecodeCompressed(compressed)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decompressed point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decompressed point should equal the y-coord of the intial point")
}

func TestIsHigherY(t *testing.T) {
	_, Ga, err := RandomG1(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	Ga.p.MakeAffine()
	GaYString := Ga.p.y.String()
	GaYBig := new(big.Int)
	_, ok := GaYBig.SetString(GaYString, 16)
	assert.True(t, ok, "ok should be True")

	GaNeg := new(G1)
	GaNeg.Neg(Ga)
	GaNeg.p.MakeAffine()
	GaNegYString := GaNeg.p.y.String()
	GaNegYBig := new(big.Int)
	_, ok = GaNegYBig.SetString(GaNegYString, 16)
	assert.True(t, ok, "ok should be True")

	// Verify that Ga.p.y + GaNeg.p.y == 0
	sumYs := &gfP{}
	fieldZero := newGFp(0)
	gfpAdd(sumYs, &Ga.p.y, &GaNeg.p.y)
	assert.Equal(t, *sumYs, *fieldZero, "The y-coordinates of P and -P should add up to zero")

	// Find which point between Ga and GaNeg is the one witht eh higher Y
	res := gfpCmp_p(&GaNeg.p.y, &Ga.p.y)
	if res > 0 { // GaNeg.p.y > Ga.p.y
		assert.True(t, GaNeg.IsHigherY(), "GaNeg.IsHigherY should be true if GaNeg.p.y > Ga.p.y")
		// Test the comparision of the big int also, should be the same result
		assert.Equal(t, GaNegYBig.Cmp(GaYBig), 1, "GaNegYBig should be bigger than GaYBig")
	} else if res < 0 { // GaNeg.p.y < Ga.p.y
		assert.False(t, GaNeg.IsHigherY(), "GaNeg.IsHigherY should be false if GaNeg.p.y < Ga.p.y")
		// Test the comparision of the big int also, should be the same result
		assert.Equal(t, GaYBig.Cmp(GaNegYBig), 1, "GaYBig should be bigger than GaNegYBig")
	}
}

func TestGetYFromMontEncodedX(t *testing.T) {
	// We know that the generator of the curve is P = (x: 1, y: 2, z: 1, t: 1)
	// We take x = 1 and we see if we retrieve P such that y = 2 or -P such that y' = Inv(2)

	// Create the GFp element 1 and MontEncode it
	PxMontEncoded := newGFp(1)
	yRetrieved, err := getYFromMontEncodedX(PxMontEncoded)
	assert.Nil(t, err)

	smallYMontEncoded := newGFp(2)
	bigYMontEncoded := &gfP{}
	gfpNeg(bigYMontEncoded, smallYMontEncoded)

	testCondition := (*yRetrieved == *smallYMontEncoded) || (*yRetrieved == *bigYMontEncoded)
	assert.True(t, testCondition, "The retrieved Y should either equal 2 or Inv(2)")
}

func TestEncodeUncompressed(t *testing.T) {
	// Case1: Create random point (Jacobian form)
	_, GaInit, err := RandomG1(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Affine form of GaInit
	GaAffine := new(G1)
	GaAffine.Set(GaInit)
	GaAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeUncompress function
	GaCopy1 := new(G1)
	GaCopy1.Set(GaInit)
	encoded := GaCopy1.EncodeUncompressed()

	// Encode GaCopy2 with the Marshal function
	GaCopy2 := new(G1)
	GaCopy2.Set(GaInit)
	marshalled := GaCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		encoded[1:], // Ignore the masking byte
		marshalled[:],
		"The EncodeUncompressed and Marshal function yield different results")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 := new(G1)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeUncompress function
	Gb2 := new(G1)
	err = Gb2.DecodeUncompressed(encoded)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decoded point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decoded point should equal the y-coord of the intial point")

	// Case2: Encode the point at infinity
	GInfinity := new(G1)
	GInfinity.p = &curvePoint{}
	GInfinity.p.SetInfinity()

	// Get the point in affine form
	GInfinityAffine := new(G1)
	GInfinityAffine.Set(GInfinity)
	GInfinityAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeUncompress function
	GInfinityCopy1 := new(G1)
	GInfinityCopy1.Set(GInfinity)
	encoded = GInfinityCopy1.EncodeUncompressed()

	// Encode GaCopy2 with the Marshal function
	GInfinityCopy2 := new(G1)
	GInfinityCopy2.Set(GInfinity)
	marshalled = GInfinityCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		encoded[1:], // Ignore the masking byte
		marshalled[:],
		"The EncodeUncompressed and Marshal function yield different results")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 = new(G1)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeCompress function
	Gb2 = new(G1)
	err = Gb2.DecodeUncompressed(encoded)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decompressed point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decompressed point should equal the y-coord of the intial point")
}
