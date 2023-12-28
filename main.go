package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/fatih/color"

	"github.com/ChaosNyaruko/ondict/render"
	"github.com/ChaosNyaruko/ondict/sources"
)

var version = "v0.0.2"
var dialTimeout = 5 * time.Second
var defaultIdleTimeout = 876000 * time.Hour // 100 years

var help = flag.Bool("h", false, "Show this help doc")
var ver = flag.Bool("version", false, "Show current version of ondict")
var word = flag.String("q", "", "Specify the word that you want to query")

// var easyMode = flag.Bool("e", false, "True to show only 'frequent' meaning")
var dev = flag.Bool("d", false, "If specified, a static html file will be parsed, instead of an online query, just for dev debugging")
var verbose = flag.Bool("v", false, "Show debug logs")
var interactive = flag.Bool("i", false, "Launch an interactive CLI app")
var server = flag.Bool("serve", false, "Serve as a HTTP server, default on UDS, for cache stuff, make it quicker!")
var idleTimeout = flag.Duration("listen.timeout", defaultIdleTimeout, "Used with '-serve', the server will automatically shut down after this duration if no new requests come in")
var listenAddr = flag.String("listen", "", "Used with '-serve', address on which to listen for remote connections. If prefixed by 'unix;', the subsequent address is assumed to be a unix domain socket. Otherwise, TCP is used.")
var remote = flag.String("remote", "", "Connect to a remote address to get information, 'auto' means it will try to launch a request by UDS. If no local server is working, a new server will be created, with -listen.timeout 1 min.")
var colour = flag.Bool("color", false, "This flags controls whether to use colors.")
var renderFormat = flag.String("f", "", "render format, 'md' (for markdown, only for mdx engine now), or 'html'")
var engine = flag.String("e", "", "query engine, 'mdx' or others(online query)")

// TODO: prev work, for better source abstractions
var g = sources.G

func main() {
	flag.Parse()
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if !*verbose {
		log.SetOutput(io.Discard)
		// TODO: they should be bound with a renderer?
		render.SeparatorOpen, render.SeparatorClose = "", ""
	}
	// TODO: put it in a better place.
	sources.LoadConfig()

	if *renderFormat != "md" {
		sources.Gbold, sources.Gitalic = "", ""
	}

	if *ver {
		fmt.Printf("ondict %s %s %s with %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}

	if !*colour {
		color.NoColor = true
	}

	if *interactive {
		g.Load()
		startLoop()
		return
	}

	if *server {
		stop := make(chan error)
		p := new(proxy)
		if *idleTimeout > 0 {
			p.timeout = time.NewTimer(*idleTimeout)
		}
		network, addr := ParseAddr(*listenAddr)
		if network == "auto" || addr == "" {
			dp, err := os.Executable()
			if err != nil {
				log.Fatalf("getting ondict path error: %v", err)
			}
			network, addr = autoNetworkAddressPosix(dp, "")
			if _, err := os.Stat(addr); err == nil {
				if err := os.Remove(addr); err != nil {
					log.Fatalf("removing remote socket file: %v", err)
				}
			}
		}
		log.Printf("start a new server: %s/%s/%s/%s", network, addr, *renderFormat, *engine)
		g.Load()
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

	var netConn net.Conn
	var err error
	var network, address string
	if *remote == "auto" {
		dp, err := os.Executable()
		if err != nil {
			log.Fatalf("getting ondict path error: %v", err)
		}
		network, address = autoNetworkAddressPosix(dp, "")
		log.Printf("auto mode dp: %v, network: %v, address: %v", dp, network, address)
		netConn, err = net.DialTimeout(network, address, dialTimeout)

		if err == nil { // detect an exsitng server, just forward a request
			if err := request(netConn, *engine, *renderFormat); err != nil {
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
			"-listen.timeout=2m",
			"-e=" + *engine,
			"-f=" + *renderFormat,
		}
		log.Printf("starting remote: %v", args)
		if err := startRemote(dp, args...); err != nil {
			log.Fatal(err)
		}
	} else {
		network, address = ParseAddr(*remote)
	}
	const retries = 5
	// It can take some time for the newly started server to bind to our address,
	// so we retry for a bit.
	for retry := 0; retry < retries; retry++ {
		startDial := time.Now()
		netConn, err = net.DialTimeout(network, address, dialTimeout)
		if err == nil {
			if err := request(netConn, *engine, *renderFormat); err != nil {
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
	log.Fatalf("failed after %d attempts", retries)

	// just for offline test.
	if *dev {
		fd, err := os.Open("./tmp/doctor_ldoce.html")
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		fmt.Println(render.ParseHTML(fd))
		return
	}

	if *engine == "mdx" {
		// io.Copy(os.Stdout, fd)
		g.Load()
		fmt.Println(sources.QueryMDX(*word, *renderFormat))
		return
	}
	fmt.Println(sources.QueryByURL(*word))
}

func query(word string, e string, f string) string {
	if e == "" {
		e = *engine
	}
	if f == "" {
		f = *renderFormat
	}
	if e == "mdx" {
		return sources.QueryMDX(word, f)
	}
	return sources.GetFromLDOCE(word)
}

func Restore() {
	sources.Restore()
}

func Store() {
	sources.Store()
}
