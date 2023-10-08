package main

import (
	"log"
	"net/http"
	"time"
)

type proxy struct {
	timeout *time.Timer
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.timeout.Reset(idleTimeout)
	q := r.URL.Query()
	word := q.Get("query")
	log.Printf("query HTTP: %v", word)

	res := query(word)
	w.Write([]byte(res))
}
