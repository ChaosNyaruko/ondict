package main

import (
	"log"
	"net/http"
	"sync"
)

type proxy struct {
	mu      sync.Mutex
	history map[string]string
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	word := q.Get("query")
	log.Printf("query HTTP: %v", word)
	s.mu.Lock()
	var res string
	if ex, ok := s.history[word]; ok {
		log.Printf("cache hit!")
		res = ex
	} else {
		res = queryByURL(word)
		s.history[word] = res
	}
	s.mu.Unlock() // TODO: performance

	w.Write([]byte(res))
}
