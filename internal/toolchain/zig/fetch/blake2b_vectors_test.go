package fetch

import (
	"encoding/hex"
	"testing"
)

func TestBlake2bVectors(t *testing.T) {
	multiBlock := make([]byte, 300)
	for i := range multiBlock {
		multiBlock[i] = byte((i*7 + 3) % 256)
	}
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte(""), "786a02f742015903c6c6fd852552d272912f4740e15847618a86e217f71f5419d25e1031afee585313896444934eb04b903a685b1448b755d56f701afe9be2ce"},
		{[]byte("abc"), "ba80a53f981c4d0d6a2797b69f12f6e94c212f14685ac4b74b12bb6fdbffa2d17d87c5392aab792dc252d5de4533cc9518d38aa8dbf1925ab92386edd4009923"},
		{multiBlock, "ec8fc8cb1343d5b8744a6971d9ae3b5463789bae1df407dadc65f7781d169a71596bc87f01691a81868f49acd6ddfbfd9874ab19eae83d40a505f6ca9db6af9f"},
	}
	for _, c := range cases {
		got := Blake2b512Sum(c.in)
		if hex.EncodeToString(got[:]) != c.want {
			t.Fatalf("blake2b(%q) = %x, want %s", c.in, got, c.want)
		}
	}
}
