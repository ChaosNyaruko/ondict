package main

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/util"
)

type proxy struct {
	timeout *time.Timer
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.timeout.Stop() {
		select {
		case t := <-s.timeout.C: // try to drain from the channel
			log.Debugf("drained from timer: %v", t)
		default:
		}
	}
	if *idleTimeout > 0 {
		s.timeout.Reset(*idleTimeout)
	}
	log.Debugf("query HTTP path: %v", r.URL.Path)
	if r.URL.Path == "/" {
		tmplt := template.New("portal")
		tmplt, err := tmplt.Parse(portal)
		if err != nil {
			log.Fatalf("parse portal html err: %v", err)
		}

		if err := tmplt.Execute(w, nil); err != nil {
			return
		}
		return
	}
	if strings.HasSuffix(r.URL.Path, "/dict") {
		q := r.URL.Query()
		word := q.Get("query")
		e := q.Get("engine")
		f := q.Get("format")
		r := q.Get("record")
		log.Debugf("query dict: %v, engine: %v, format: %v", word, e, f)

		res := query(word, e, f, r != "0")
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(res))
		w.(http.Flusher).Flush()
		// w.Write([]byte("<style>" + odecss + "</style>"))
		// w.Write([]byte(fmt.Sprintf(`<link ref="stylesheet" type="text/css", href=/d/static/oald9.css />`)))
		return
	}
	// if strings.HasSuffix(r.URL.Path, ".css") {
	// 	log.Debugf("static info: %v", r.URL.Path)
	// 	http.FileServer(http.Dir("./static")).ServeHTTP(w, r)
	// 	return
	// }
	log.Infof("URL: %v, Scheme: %v", r.URL, r.URL.Scheme)
	http.FileServer(http.Dir(util.TmpDir())).ServeHTTP(w, r)
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
