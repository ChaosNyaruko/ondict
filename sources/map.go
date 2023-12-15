package sources

type Map map[string]string

func (m Map) Get(word string) string {
	return m[word]
}

func (m Map) Keys() []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}

	return res
}
