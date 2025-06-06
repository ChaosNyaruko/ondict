package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/fzf"
	"github.com/ChaosNyaruko/ondict/history"
	"github.com/ChaosNyaruko/ondict/render"
	"github.com/ChaosNyaruko/ondict/sources"
)

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "no-vcs.revision(go build -buildvcs)"
}()

var Version = "v0.1.2"

var dialTimeout = 5 * time.Second
var defaultIdleTimeout = 876000 * time.Hour // 100 years

var help = flag.Bool("h", false, "Show this help doc")
var ver = flag.Bool("version", false, "Show current version of ondict")
var word = flag.String("q", "", "Specify the word that you want to query")

var record = flag.Int("r", 0, "Specify this query should be recorded in the log, only take effect with -q. \n0: Not recording\n1: Record it locally \n2: Tell the remote server to record it\n3: Record on both sides (If there is a -remote specified)")

// var easyMode = flag.Bool("e", false, "True to show only 'frequent' meaning")
var dev = flag.Bool("d", false, "If specified, a static html file will be parsed, instead of an online query, just for dev debugging")
var verbose = flag.Bool("v", false, "Show debug logs")
var interactive = flag.Bool("i", false, "Launch an interactive CLI app")
var useFzf = flag.Bool("fzf", false, "EXPERIMENTAL: whether to use fzf as the fuzzy search tool")
var ahoFuzzy = flag.Bool("aho", false, "When enabled, searching for something will use 'aho-corasick' algorithm, which will cost much more memory, \nbut allows you to find SHORTER && SIMILAR results when you didn't type in the exact word existing in the MDX dictionaries, \ni.e. finding the LONGEST match in the MDX dictionaries. \nNOT take effect when '-fzf' is enabled.")
var dumpMDD = flag.Bool("dump", false, "If true, it will re-dump the mdd data when launched. The dumping will be running in the background, so the server won't be stuck")
var lazy = flag.Bool("lazy", true, "If disabled(-lazy=false), the application will load all the dictionary items in the MDX file, rather than loading them at the first query.")
var server = flag.Bool("serve", false, "Serve as a HTTP server, default on UDS, for cache stuff, make it quicker!")
var idleTimeout = flag.Duration("listen.timeout", defaultIdleTimeout, "Used with '-serve', the server will automatically shut down after this duration if no new requests come in")
var listenAddr = flag.String("listen", "", "Used with '-serve', address on which to listen for remote connections. If prefixed by 'unix;', the subsequent address is assumed to be a unix domain socket. Otherwise, TCP is used.")
var remote = flag.String("remote", "", "Connect to a remote address to get information, 'auto' means it will try to launch a request by UDS. If no local server is working, a new server will be created, with -listen.timeout 2min.")
var colour = flag.Bool("color", false, "This flags controls whether to use colors.")
var renderFormat = flag.String("f", "", "render format, 'md' (for markdown, only for mdx engine now), or 'html'")
var engine = flag.String("e", "", "query engine, 'mdx' or others(online query)")

// TODO: prev work, for better source abstractions
var g = sources.G

var his *history.History

func init() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.InfoLevel)
}

func main() {
	flag.Parse()
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}

	if *ver {
		fmt.Printf("ondict version: %s-%s built on %s %s with %s\n", Version, Commit, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	his = history.NewHistory(history.NewTxtWriter(), history.NewSqlite3Writer())

	if !*verbose {
		log.SetLevel(log.InfoLevel)
		// TODO: they should be bound with a renderer?
		render.SeparatorOpen, render.SeparatorClose = "", ""
	} else {
		log.SetLevel(log.DebugLevel)
	}

	if *renderFormat != "md" {
		sources.Gbold, sources.Gitalic = "", ""
	}

	if !*colour {
		color.NoColor = true
	}

	if *useFzf {
		g.Load(true, false, *lazy)
		fzf.ListAllWord()
		return
	}

	if *interactive {
		g.Load(!*ahoFuzzy, *dumpMDD, *lazy)
		startLoop()
		return
	}

	if *server {
		go http.ListenAndServe("localhost:8083", nil)
		stop := make(chan error)
		p := NewProxy()
		if *idleTimeout > 0 {
			p.timeout = time.NewTimer(*idleTimeout)
		}
		network, addr := ParseAddr(*listenAddr)
		if network == "auto" || addr == "" {
			dp, err := os.Executable()
			if err != nil {
				log.Fatalf("getting ondict path error: %v", err)
			}
			network, addr = autoNetworkAddress(dp, "")
			if _, err := os.Stat(addr); err == nil {
				if err := os.Remove(addr); err != nil {
					log.Fatalf("removing remote socket file: %v", err)
				}
			}
		}
		log.Debugf("start a new server: %s/%s/%s/%s/%v", network, addr, *renderFormat, *engine, *dumpMDD)
		g.Load(!*ahoFuzzy, *dumpMDD, *lazy)
		l, err := net.Listen(network, addr)
		if err != nil {
			log.Fatal("bad Listen: ", err)
		}

		go func() {
			if err := p.Run(l); err != nil {
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

	// one shot mode (-q word)
	if *remote != "" {
		var netConn net.Conn
		var err error
		var network, address string
		if *remote == "auto" {
			dp, err := os.Executable()
			if err != nil {
				log.Fatalf("getting ondict path error: %v", err)
			}
			network, address = autoNetworkAddress(dp, "")
			log.Debugf("auto mode dp: %v, network: %v, address: %v", dp, network, address)
			netConn, err = net.DialTimeout(network, address, dialTimeout)

			if err == nil { // detect an exsitng server, just forward a request
				if err := request(address, netConn, *engine, *renderFormat, *record); err != nil {
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
			log.Debugf("starting remote: %v", args)
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
			log.Debugf("dialling %v %v", network, address)
			startDial := time.Now()
			netConn, err = net.DialTimeout(network, address, dialTimeout)
			if err == nil {
				if err := request(address, netConn, *engine, *renderFormat, *record); err != nil {
					log.Fatal(err)
				}
				return
			}
			log.Debugf("failed attempt #%d to connect to remote: %v\n", retry+2, err)
			// In case our failure was a fast-failure, ensure we wait at least
			// f.dialTimeout before trying again.
			if retry != retries-1 {
				time.Sleep(dialTimeout - time.Since(startDial))
			}
		}
		log.Fatalf("failed after %d attempts, remote: %v", retries, *remote)
	}

	// one-shot query, without making a request to "remote", remote is empty
	if *engine == "mdx" {
		g.Load(!*ahoFuzzy, *dumpMDD, *lazy)
	}
	fmt.Println(query(*word, *engine, *renderFormat, *record&0x1 != 0))
}

func query(word string, e string, f string, r bool) string {
	if r {
		if err := his.Append(word); err != nil {
			log.Debugf("record %v err: %v", word, err)
		}
	}
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
