package main

import (
	"log"
	"net/http"
)

type proxy struct {
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	word := q.Get("query")
	log.Printf("query HTTP: %v", word)

	res := query(word)
	w.Write([]byte(res))
}
