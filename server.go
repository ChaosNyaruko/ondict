package main

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type proxy struct {
	timeout *time.Timer
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.timeout.Stop() {
		select {
		case t := <-s.timeout.C: // try to drain from the channel
			log.Printf("drained from timer: %v", t)
		default:
		}
	}
	if *idleTimeout > 0 {
		s.timeout.Reset(*idleTimeout)
	}
	log.Printf("query HTTP path: %v", r.URL.Path)
	if r.URL.Path == "/dict" {
		q := r.URL.Query()
		word := q.Get("query")
		e := q.Get("engine")
		f := q.Get("format")
		log.Printf("query HTTP: %v, engine: %v, format: %v", word, e, f)

		res := query(word, e, f)
		w.Write([]byte(res))
	}
	http.FileServer(http.Dir(".")).ServeHTTP(w, r)
}

func ParseAddr(listen string) (network string, address string) {
	// Allow passing just -remote=auto, as a shorthand for using automatic remote
	// resolution.
	if listen == "auto" {
		return "auto", ""
	}
	if parts := strings.SplitN(listen, ";", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "tcp", listen
}
