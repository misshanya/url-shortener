package base62

import "testing"

type pair struct {
	num     int64
	encoded string
}

var pairs = []pair{
	{1, "1"},
	{2, "2"},
	{10, "A"},
	{222, "3a"},
	{951, "FL"},
}

func TestEncode(t *testing.T) {
	for _, pair := range pairs {
		if out := Encode(pair.num); out != pair.encoded {
			t.Errorf("Output %q is not equal to excepted %q", out, pair.encoded)
		}
	}
}

func TestDecode(t *testing.T) {
	for _, pair := range pairs {
		if out := Decode(pair.encoded); out != pair.num {
			t.Errorf("Output %d is not equal to excepted %d", out, pair.num)
		}
	}
}
