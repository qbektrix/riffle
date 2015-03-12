package lib

import (
	"bytes"
	"errors"
	goCipher "crypto/cipher"
	"crypto/aes"
	"encoding/binary"
	"log"
	"time"
	"os"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/cipher"
	"github.com/dedis/crypto/random"
)

func SetBit(n_int int, b bool, bs []byte) {
	n := uint(n_int)
	if b {
		bs[n/8] |= 1 << (n % 8)
	} else {
		bs[n/8] &= ^(1 << (n % 8))
	}
}

func Xor(r []byte, w []byte) {
	for j := 0; j < len(w)/len(r); j+=len(r) {
		for i, b := range r {
			w[i] ^= b
		}
	}
}

func Xors(bss [][]byte) []byte {
	n := len(bss[0])
	x := make([]byte, n)
	for _, bs := range bss {
		for i, b := range bs {
			x[i] ^= b
		}
	}
	return x
}

func XorsDC(bsss [][][]byte) [][]byte {
	n := len(bsss)
	m := len(bsss[0])
	x := make([][]byte, n)
	for i, _ := range bsss {
		y := make([][]byte, m)
		for j := 0; j < m; j++ {
			y[j] = bsss[j][i]
		}
		x[i] = Xors(y)
	}
	return x
}

func AllZero(xs []byte) bool {
	for _, x := range xs {
		if x != 0 {
			return false
		}
	}
	return true
}

func ComputeResponse(allBlocks []Block, mask []byte, secret []byte) []byte {
	response := make([]byte, BlockSize)
        i := 0
L:
        for _, b := range mask {
                for j := 0; j < 8; j++ {
                        if b&1 == 1 {
                                Xor(allBlocks[i].Block, response)
                        }
                        b >>= 1
                        i++
                        if i >= len(allBlocks) {
                                break L
                        }
                }
        }
	Xor(secret, response)
        return response
}

func SliceEquals(X, Y []byte) bool {
	if len(X) != len(Y) {
		return false
	}
	for i := range X {
		if X[i] != Y[i] {
			return false
		}
	}
	return true
}

func GeneratePI(size int, rand cipher.Stream) []int {
	// Pick a random permutation
	pi := make([]int, size)
	for i := 0; i < size; i++ {	// Initialize a trivial permutation
		pi[i] = i
	}
	for i := size-1; i > 0; i-- {	// Shuffle by random swaps
		j := int(random.Uint64(rand) % uint64(i+1))
		if j != i {
			t := pi[j]
			pi[j] = pi[i]
			pi[i] = t
		}
	}
	return pi
}

func Encrypt(g abstract.Group, msg []byte, pks []abstract.Point) ([]abstract.Point, []abstract.Point) {
	c1s := []abstract.Point{}
	c2s := []abstract.Point{}
	var msgPt abstract.Point
	remainder := msg
	for ; len(remainder) != 0 ;  {
		msgPt, remainder = g.Point().Pick(remainder, random.Stream)
		k := g.Secret().Pick(random.Stream)
		c1 := g.Point().Mul(nil, k)
		var c2 abstract.Point = nil
		for _, pk := range pks {
			if c2 == nil {
				c2 = g.Point().Mul(pk, k)
			} else {
				c2 = c2.Add(c2, g.Point().Mul(pk, k))
			}
		}
		c2 = c2.Add(c2, msgPt)
		c1s = append(c1s, c1)
		c2s = append(c2s, c2)
	}
	return c1s, c2s
}

func EncryptPoint(g abstract.Group, msgPt abstract.Point, pk abstract.Point) (abstract.Point, abstract.Point) {
	k := g.Secret().Pick(random.Stream)
	c1 := g.Point().Mul(nil, k)
	c2 := g.Point().Mul(pk, k)
	c2 = c2.Add(c2, msgPt)
	return c1, c2
}

func Decrypt(g abstract.Group, c1 abstract.Point, c2 abstract.Point, sk abstract.Secret) abstract.Point {
	return g.Point().Sub(c2, g.Point().Mul(c1, sk))
}

func CounterAES(key []byte, block []byte) []byte {
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal("Could not create encryptor")
	}

	ciphertext := make([]byte, len(block))
	var counter uint64 = 0
	iv := make([]byte, aes.BlockSize)
	binary.PutUvarint(iv, counter)

	stream := goCipher.NewCTR(aesCipher, iv)
	stream.XORKeyStream(ciphertext, block)

	return ciphertext
}

func Membership(res []byte, set [][]byte) int {
	for i := range set {
		same := true
		same = same && (len(res) == len(set[i]))
		for k := range res {
			same = same && (set[i][k] == res[k])
		}
		if same {
			return i
		}
	}
	return -1
}

func MarshalPoint(pt abstract.Point) []byte {
	buf := new(bytes.Buffer)
	ptByte := make([]byte, SecretSize)
	pt.MarshalTo(buf)
	buf.Read(ptByte)
	return ptByte
}

func UnmarshalPoint(ptByte []byte) abstract.Point {
	buf := bytes.NewBuffer(ptByte)
	pt := Suite.Point()
	pt.UnmarshalFrom(buf)
	return pt
}

func RunFunc(f func(int)) {
	for r := 0; r < MaxRounds; r++ {
		go func (r int) {
			for {
				f(r)
			}
		} (r)
	}
}

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

func NewDesc(path string) (map[string]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal("Failed opening file", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() % HashSize != 0 {
		return nil, errors.New("Misformatted file")
	}
	numHashes := fi.Size() / HashSize

	hashes := make(map[string]int64)

	for i := 0; int64(i) < numHashes; i++ {
		hash := make([]byte, HashSize)
		_, err := f.Read(hash)
		if err != nil {
			log.Fatal("Failed reading file", err)
		}
		hashes[string(hash)] = int64(i * BlockSize)
	}

	return hashes, nil
}

func NewFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal("Failed opening file", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	blocks := (fi.Size() + BlockSize - 1) / BlockSize

	x := &File{
		Name: path,
		Hashes: make(map[string]int64, blocks),
	}

	for i := 0; int64(i) < blocks; i++ {
		tmp := make([]byte, BlockSize)
		_, err := f.Read(tmp)
		if err != nil {
			log.Fatal("Failed reading file", err)
		}
		h := Suite.Hash()
		h.Write(tmp)
		x.Hashes[string(h.Sum(nil))] = int64((i * BlockSize))
	}

	return x, nil
}
