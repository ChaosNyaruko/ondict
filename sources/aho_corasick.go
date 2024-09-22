// Aho–Corasick algorithm
// Refer to https://en.wikipedia.org/wiki/Aho%E2%80%93Corasick_algorithm
// https://www.youtube.com/watch?v=O7_w001f58c
// https://www.youtube.com/watch?v=OFKxWFew_L0
package sources

import (
	"strings"

	ahocorasick "github.com/BobuSumisu/aho-corasick"
	log "github.com/sirupsen/logrus"
)

type Dict interface {
	Keys() []string
	Get(string) string
}

type AhoCorasick struct {
	dict    Dict
	lowDict map[string][]string
	trie    *ahocorasick.Trie
}

func NewAho(dict Dict) Searcher {
	keys := dict.Keys()
	// log.Debugf("new aho_corasick: %v", keys)
	// lowercase
	lowDict := make(map[string][]string, len(keys))
	for _, k := range keys {
		lk := strings.ToLower(k)
		lowDict[lk] = append(lowDict[lk], k)
	}

	input := make([]string, 0, len(keys))
	for k := range lowDict {
		input = append(input, k)
	}
	log.Debugf("raw dict %d items, "+
		"lowercase dict %d items, "+
		"because different item in the raw dictionary "+
		"like 'August' and 'august' will be "+
		"combined into a string slice\n",
		len(keys), len(lowDict))
	trie := ahocorasick.NewTrieBuilder().AddStrings(input).Build()

	return &AhoCorasick{dict: dict, trie: trie, lowDict: lowDict}
}

func (ack *AhoCorasick) GetRawOutputs(input string) []RawOutput {
	matches := ack.trie.Match([]byte(input))
	res := make([]RawOutput, 0, len(matches))
	for i, match := range matches {
		log.Debugf("%d th match: pos[%v], pattern[%v], string[%v]\n", i, match.Pos(), match.Pattern(), match.MatchString())
		for _, v := range ack.lowDict[match.MatchString()] {
			res = append(res, output{v, ack.dict.Get(v)})
		}
	}
	return res
}
