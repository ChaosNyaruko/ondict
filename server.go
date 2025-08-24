package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/util"
)

type proxy struct {
	e       *gin.Engine
	timeout *time.Timer
}

func (p *proxy) Run(l net.Listener) error {
	return p.e.RunListener(l)
}

func review(c *gin.Context) {
	days, x := c.GetQuery("days_ago")
	count, y := c.GetQuery("count")
	if !x && !y {
		tmplt := template.New("review")
		tmplt, err := tmplt.Parse(reviewPage)
		if err != nil {
			log.Fatalf("parse portal html err: %v", err)
		}

		if err := tmplt.Execute(c.Writer, nil); err != nil {
			return
		}
		return
	}
	words, err := his.Review(days, count)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("bad review request: %v", err))
	}
	c.String(200, "%v", words)
}

func queryWord(c *gin.Context) {
	word, _ := c.GetQuery("query")
	e, _ := c.GetQuery("engine")
	f, _ := c.GetQuery("format")
	r, _ := c.GetQuery("record")

	res := query(word, e, f, r != "0")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(res))
	return
}

func index(c *gin.Context) {
	tmplt := template.New("portal")
	tmplt, err := tmplt.Parse(portal)
	if err != nil {
		log.Fatalf("parse portal html err: %v", err)
	}

	if err := tmplt.Execute(c.Writer, nil); err != nil {
		return
	}
	return
}

func NewProxy() *proxy {
	r := gin.Default()
	// r.LoadHTMLGlob("templates/*")
	// Set up cookie-based sessions
	store := cookie.NewStore([]byte("secret-key"))
	r.Use(sessions.Sessions("session", store))

	r.GET("/login", loginHandler)
	r.POST("/login", processLogin)
	r.GET("/auth", authMiddleware(), review)

	r.GET("/", index)
	r.Use(static.Serve("/", static.LocalFile(util.TmpDir(), false)))
	r.GET("/dict", queryWord)
	r.GET("/review", review)
	r.GET("/complete", completeHandler)
	return &proxy{
		e: r,
	}
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
	s.e.ServeHTTP(w, r)
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

func completeHandler(c *gin.Context) {
	prefix, pre := c.GetQuery("prefix")
	if !pre {
		c.String(200, "prefix empty")
		return
	}

	var words []string
	for _, g := range *sources.G {
		words = append(words, g.MdxDict.Keys()...)
	}

	var suggestions []string
	for _, word := range words {
		if strings.HasPrefix(strings.ToLower(word), strings.ToLower(prefix)) {
			suggestions = append(suggestions, word)
			if len(suggestions) >= 10 {
				break
			}
		}
	}

	res, err := json.Marshal(suggestions)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("unmarshal error"))
		return
	}
	c.Data(200, "application/json", res)
}
