package sources

import (
	"os"
	"strings"

	"github.com/jwangsadinata/go-multimap"
	"github.com/jwangsadinata/go-multimap/slicemultimap"
	"github.com/schollz/progressbar/v3"
)

// IExact is a case in-sensitive search source, no other fuzzy searching algos.
type IExact struct {
	keys multimap.MultiMap
	dict Dict
}

var _ Searcher = &IExact{}

func NewIExact(dict Dict) Searcher {
	keys := slicemultimap.New()
	total := len(dict.Keys())
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription("constructing iexact searcher"),
	)

	for _, k := range dict.Keys() {
		lk := strings.ToLower(k)
		keys.Put(lk, k)
		bar.Add(1)
	}
	return &IExact{dict: dict, keys: keys}
}

func (e *IExact) GetRawOutputs(input string) []RawOutput {
	res := make([]RawOutput, 0, 1)
	ro, _ := e.keys.Get(strings.ToLower(input))

	for _, r := range ro {
		def := e.dict.Get(r.(string))
		res = append(res, output{
			rawWord: input,
			def:     def,
		})
	}
	return res
}
