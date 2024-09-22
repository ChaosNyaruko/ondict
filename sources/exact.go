package sources

type Exact struct {
	dict Dict
}

func NewExact(dict Dict) Searcher {
	return &Exact{dict: dict}
}

func (e *Exact) GetRawOutputs(input string) []RawOutput {
	res := make([]RawOutput, 0, 1)
	res = append(res, output{
		rawWord: input,
		def:     e.dict.Get(input),
	})
	return res
}
