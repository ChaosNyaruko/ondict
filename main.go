package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/fatih/color"
)

var version = "v0.0.1"
var dialTimeout = 5 * time.Second
var defaultIdleTimeout = 1 * time.Minute

var help = flag.Bool("h", false, "Show this help doc")
var ver = flag.Bool("version", false, "Show current version of ondict")
var word = flag.String("q", "", "Specify the word that you want to query")
var easyMode = flag.Bool("e", false, "True to show only 'frequent' meaning")
var dev = flag.Bool("d", false, "If specified, a static html file will be parsed, instead of an online query, just for dev debugging")
var verbose = flag.Bool("v", false, "Show debug logs")
var interactive = flag.Bool("i", false, "Launch an interactive CLI app")
var server = flag.Bool("serve", false, "Serve as a HTTP server, default on UDS, for cache stuff, make it quicker!")
var idleTimeout = flag.Duration("listen.timeout", defaultIdleTimeout, "Used with '-serve', the server will automatically shut down after this duration if no new requests come in")
var remote = flag.String("remote", "", "Connect to a remote address to get information, 'auto' means it will try to launch a request by UDS. If no local server is working, a new server will be created, with -listen.timeout 1 min.")
var colour = flag.Bool("color", false, "This flags controls whether to use colors.")

var mu sync.Mutex // owns history
var history map[string]string = make(map[string]string)
var dataPath string
var historyFile string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	config := filepath.Join(home, ".config")
	dataPath = filepath.Join(config, "ondict")
	historyFile = filepath.Join(dataPath, "history.json")
	if dataPath == "" || historyFile == "" {
		log.Fatalf("empty datapath/historyfile: %v||%v", dataPath, historyFile)
	}
}

func main() {
	flag.Parse()
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if !*verbose {
		log.SetOutput(io.Discard)
	}
	if *ver {
		fmt.Printf("ondict %s %s %s with %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}

	if !*colour {
		color.NoColor = true
	}

	if *interactive {
		startLoop()
		return
	}

	if *server {
		stop := make(chan error)
		p := new(proxy)
		p.timeout = time.NewTimer(*idleTimeout)
		dp, err := os.Executable()
		if err != nil {
			log.Fatalf("getting ondict path error: %v", err)
		}
		network, addr := autoNetworkAddressPosix(dp, "")
		if _, err := os.Stat(addr); err == nil {
			if err := os.Remove(addr); err != nil {
				log.Fatalf("removing remote socket file: %v", err)
			}
		}
		log.Printf("%s, start a new server: %s", dp, addr)
		l, err := net.Listen(network, addr)
		if err != nil {
			log.Fatal("bad Listen: ", err)
		}
		server := http.Server{
			Handler: p,
		}

		go func() {
			if err := server.Serve(l); err != nil {
				stop <- err
				close(stop)
			}
		}()

		select {
		case c := <-p.timeout.C:
			log.Fatal("timeout, server down!", c)
		case err := <-stop:
			log.Fatal("server down", err)
		}
	}

	if *remote == "auto" {
		dp, err := os.Executable()
		if err != nil {
			log.Fatalf("getting ondict path error: %v", err)
		}
		network, address := autoNetworkAddressPosix(dp, "")
		log.Printf("auto mode dp: %v, network: %v, address: %v", dp, network, address)
		netConn, err := net.DialTimeout(network, address, dialTimeout)

		if err == nil { // detect an exsitng server, just forward a request
			if err := request(netConn); err != nil {
				log.Fatal(err)
			}
			return
		}
		if network == "unix" {
			// Sometimes the socketfile isn't properly cleaned up when the server
			// shuts down. Since we have already tried and failed to dial this
			// address, it should *usually* be safe to remove the socket before
			// binding to the address.
			// TODO(rfindley): there is probably a race here if multiple server
			// instances are simultaneously starting up.
			if _, err := os.Stat(address); err == nil {
				if err := os.Remove(address); err != nil {
					log.Fatalf("removing remote socket file: %v", err)
				}
			}
		}
		args := []string{
			"-serve=true",
		}
		log.Printf("starting remote: %v", args)
		if err := startRemote(dp, args...); err != nil {
			log.Fatal(err)
		}
		const retries = 5
		// It can take some time for the newly started server to bind to our address,
		// so we retry for a bit.
		for retry := 0; retry < retries; retry++ {
			startDial := time.Now()
			netConn, err = net.DialTimeout(network, address, dialTimeout)
			if err == nil {
				if err := request(netConn); err != nil {
					log.Fatal(err)
				}
				return
			}
			log.Printf("failed attempt #%d to connect to remote: %v\n", retry+2, err)
			// In case our failure was a fast-failure, ensure we wait at least
			// f.dialTimeout before trying again.
			if retry != retries-1 {
				time.Sleep(dialTimeout - time.Since(startDial))
			}
		}
		os.Exit(3)
	} else if *remote != "" {
		log.Fatal("TODO: specify a remote address not supported yet")
	}

	// just for offline test.
	if *dev {
		fd, err := os.Open("./tmp/doctor.html")
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		fmt.Println(parseHTML(fd))
		return
	}
	fmt.Println(queryByURL(*word))
}

func query(word string) string {
	var res string
	mu.Lock()
	if ex, ok := history[word]; ok {
		log.Printf("cache hit!")
		res = ex
	} else {
		res = queryByURL(word)
		history[word] = res
	}
	mu.Unlock() // TODO: performance
	return res
}

func Restore() {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		log.Printf("open file history err: %v", err)
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &history)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("history: %v", history)
}

func Store() {
	his, err := json.Marshal(history)
	if err != nil {
		log.Fatal("marshal err ", err)
	}
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		log.Fatal("make dir err", err)
	}
	f, err := os.Create(historyFile)
	if err != nil {
		log.Fatal("create file err", err)
	}

	defer f.Close()

	_, err = f.Write(his)

	if err != nil {
		log.Fatal("write file err", err)
	}
}
