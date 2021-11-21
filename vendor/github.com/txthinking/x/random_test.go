package x

import (
	"testing"
)

func TestRandomNumber(t *testing.T) {
	t.Log(RandomNumber())
	t.Log(RandomNumber())
	t.Log(RandomNumber())
	t.Log(Random(1000, 9999))
	t.Log(Random(1000, 9999))
}
