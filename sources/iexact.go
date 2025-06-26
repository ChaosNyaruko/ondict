package sources

import (
	"strings"

	"github.com/jwangsadinata/go-multimap"
	"github.com/jwangsadinata/go-multimap/slicemultimap"
)

// IExact is a case in-sensitive search source, no other fuzzy searching algos.
type IExact struct {
	dict multimap.MultiMap
}

var _ Searcher = &IExact{}

func NewIExact(dict Dict) Searcher {
	d := slicemultimap.New()
	for _, k := range dict.Keys() {
		lk := strings.ToLower(k)
		d.Put(lk, dict.Get(k))
	}
	return &IExact{dict: d}
}

func (e *IExact) GetRawOutputs(input string) []RawOutput {
	res := make([]RawOutput, 0, 1)
	ro, _ := e.dict.Get(strings.ToLower(input))

	for _, r := range ro {
		res = append(res, output{
			rawWord: input,
			def:     r.(string),
		})
	}
	return res
}
