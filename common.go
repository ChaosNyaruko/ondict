package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

func request(target string, netConn net.Conn, e, f string, r int) error {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return netConn, nil
			},
		},
	}
	if r&0x1 != 0 {
		if err := his.Append(*word); err != nil {
			log.Warnf("append %s to history err: %v", *word, err)
		}
	}
	scheme := "http"
	hostname := "fakedomain"
	https := false
	var p []string
	if p = strings.SplitN(target, ":", 2); len(p) == 2 {
		if p[1] == "443" {
			https = true
		} else {
			addrs, err := net.LookupHost(p[0])
			log.Debugf("addrs by lookup: %v %v", p[0], addrs)
			if err == nil && !strings.EqualFold(p[0], "localhost") {
				https = true
			}
		}
	}
	if https {
		scheme = "https"
		hostname = p[0]
	}
	res, err := httpc.Get(fmt.Sprintf("%v://%v/dict?query=%s&engine=%s&format=%s&record=%d", scheme, hostname, url.QueryEscape(*word), e, f, r&0x2))
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("new request error %v", err)
	}
	defer res.Body.Close()
	if res, err := io.ReadAll(res.Body); err != nil {
		return fmt.Errorf("read body error %v", err)
	} else {
		fmt.Println(string(res))

	}

	return nil
}
