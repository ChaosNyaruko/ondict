package main

import (
	"log"
	"os"
	"testing"

	"golang.org/x/net/html"
)

func Test_MDXParser(t *testing.T) {
	// fd, err := os.Open("./tmp/test.html")
	fd, err := os.Open("./tmp/doctor_mdx.html")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	doc, err := html.ParseWithOptions(fd, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	// log.Printf("result: %v", readText(doc))
	t.Logf("res: %v", format([]string{f(doc, 0, nil)}))
}
