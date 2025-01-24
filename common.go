package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"

	log "github.com/sirupsen/logrus"
)

func request(netConn net.Conn, e, f string, r int) error {
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
	res, err := httpc.Get(fmt.Sprintf("http://fakedomain/dict?query=%s&engine=%s&format=%s&record=%d", url.QueryEscape(*word), e, f, r&0x2))
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
