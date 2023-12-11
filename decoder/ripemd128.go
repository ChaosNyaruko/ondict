package decoder

import (
	"github.com/C0MM4ND/go-ripemd"
)

func ripemd128(m []byte) []byte {
	md := ripemd.New128()
	md.Write(m)
	return md.Sum(nil)
}
