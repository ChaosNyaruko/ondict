package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
)

func request(network, target string, netConn net.Conn, e, f string, r int) error {
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
	path := "/dict"

	https := false
	var host, port string
	var err error
	if network != "unix" {
		if host, port, err = net.SplitHostPort(target); err == nil {
			if port == "443" {
				https = true
			} else if ip := net.ParseIP(host); ip != nil {
				// raw ip
			} else {
				addrs, err := net.LookupHost(host)
				log.Debugf("addrs by lookup: %v %v", host, addrs)
				if err == nil && !strings.EqualFold(host, "localhost") {
					https = true
				}
			}
		} else {
			return err
		}
		if https {
			scheme = "https"
			hostname = host
		}
	}
	if *searchDef {
		path = "/search"
	}
	values := url.Values{}
	values.Set("query", *word)
	values.Set("engine", e)
	values.Set("format", f)
	values.Set("record", fmt.Sprintf("%d", r&0x2))
	if *searchDef {
		values.Set("mode", "definition")
	}
	res, err := httpc.Get(fmt.Sprintf("%v://%v%s?%s", scheme, hostname, path, values.Encode()))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res, err := io.ReadAll(res.Body); err != nil {
		return fmt.Errorf("read body error %v", err)
	} else {
		fmt.Println(string(res))

	}

	return nil
}
