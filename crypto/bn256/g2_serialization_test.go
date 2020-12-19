package bn256

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestG2DecodeCompressed(t *testing.T) {
	_, GaInit, err := RandomG2(rand.Reader)
	assert.NoError(t, err, "Err should be nil")

	// Affine form of GaInit
	GaAffine := new(G2)
	GaAffine.Set(GaInit)
	GaAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeCompress function
	GaCopy1 := new(G2)
	GaCopy1.Set(GaInit)
	compressed := GaCopy1.EncodeCompressed()

	// Encode GaCopy2 with the Marshal function
	GaCopy2 := new(G2)
	GaCopy2.Set(GaInit)
	marshalled := GaCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		compressed[1:],  // Ignore the masking byte
		marshalled[:64], // Get only the x-coordinate
		"The EncodeCompressed and Marshal function yield different results for the x-coordinate",
	)

	// Unmarshal the point Ga with the unmarshal function
	Gb1 := new(G2)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeCompress function
	Gb2 := new(G2)
	err = Gb2.DecodeCompressed(compressed)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decompressed point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decompressed point should equal the y-coord of the intial point")

	// == Case2: Encode the point at infinity == //
	GInfinity := new(G2)
	GInfinity.p = &twistPoint{}
	GInfinity.p.SetInfinity()

	// Get the point in affine form
	GInfinityAffine := new(G2)
	GInfinityAffine.Set(GInfinity)
	GInfinityAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeCompress function
	GInfinityCopy1 := new(G2)
	GInfinityCopy1.Set(GInfinity)
	compressed = GInfinityCopy1.EncodeCompressed()

	// Encode GaCopy2 with the Marshal function
	GInfinityCopy2 := new(G2)
	GInfinityCopy2.Set(GInfinity)
	marshalled = GInfinityCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		compressed[1:], // Ignore the masking byte
		marshalled[:64],
		"The EncodeCompressed and Marshal function yield different results")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 = new(G2)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeCompress function
	Gb2 = new(G2)
	err = Gb2.DecodeCompressed(compressed)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decompressed point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decompressed point should equal the y-coord of the intial point")
}

func TestG2DecodeUncompressed(t *testing.T) {
	// == Case1: Create random point (Jacobian form) == //
	_, GaInit, err := RandomG2(rand.Reader)
	assert.NoError(t, err, "Err should be nil")

	// Affine form of GaInit
	GaAffine := new(G2)
	GaAffine.Set(GaInit)
	GaAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeUncompress function
	GaCopy1 := new(G2)
	GaCopy1.Set(GaInit)
	encoded := GaCopy1.EncodeUncompressed()

	// Encode GaCopy2 with the Marshal function
	GaCopy2 := new(G2)
	GaCopy2.Set(GaInit)
	marshalled := GaCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		encoded[1:], // Ignore the masking byte
		marshalled[:],
		"The EncodeUncompressed and Marshal function yield different results")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 := new(G2)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeUncompress function
	Gb2 := new(G2)
	err = Gb2.DecodeUncompressed(encoded)
	assert.Nil(t, err)
	assert.Equal(t, GaAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decoded point should equal the x-coord of the intial point")
	assert.Equal(t, GaAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decoded point should equal the y-coord of the intial point")

	// == Case2: Encode the point at infinity == //
	GInfinity := new(G2)
	GInfinity.p = &twistPoint{}
	GInfinity.p.SetInfinity()

	// Get the point in affine form
	GInfinityAffine := new(G2)
	GInfinityAffine.Set(GInfinity)
	GInfinityAffine.p.MakeAffine()

	// Encode GaCopy1 with the EncodeUncompress function
	GInfinityCopy1 := new(G2)
	GInfinityCopy1.Set(GInfinity)
	encoded = GInfinityCopy1.EncodeUncompressed()

	// Encode GaCopy2 with the Marshal function
	GInfinityCopy2 := new(G2)
	GInfinityCopy2.Set(GInfinity)
	marshalled = GInfinityCopy2.Marshal() // Careful Marshal modifies the point since it makes it an affine point!

	// Make sure that the x-coordinate is encoded as it is when we call the Marshal function
	assert.Equal(
		t,
		encoded[1:], // Ignore the masking byte
		marshalled[:],
		"The EncodeUncompressed and Marshal function yield different results")

	// Unmarshal the point Ga with the unmarshal function
	Gb1 = new(G2)
	_, err = Gb1.Unmarshal(marshalled)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb1.p.x.String(), "The x-coord of the unmarshalled point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb1.p.y.String(), "The y-coord of the unmarshalled point should equal the y-coord of the intial point")

	// Decode the point Ga with the decodeCompress function
	Gb2 = new(G2)
	err = Gb2.DecodeUncompressed(encoded)
	assert.Nil(t, err)
	assert.Equal(t, GInfinityAffine.p.x.String(), Gb2.p.x.String(), "The x-coord of the decompressed point should equal the x-coord of the intial point")
	assert.Equal(t, GInfinityAffine.p.y.String(), Gb2.p.y.String(), "The y-coord of the decompressed point should equal the y-coord of the intial point")
}
