package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/internal/httpserver"
	"github.com/ChaosNyaruko/ondict/util"
)

type proxy struct {
	e       *gin.Engine
	timeout *time.Timer
}

func (p *proxy) Run(l net.Listener) error {
	log.Infof("proxy started!")
	return p.e.RunListener(l)
}

// reviewHandler wraps the shared review logic with access to the main
// package's history instance.
func reviewHandler(c *gin.Context) {
	days, x := c.GetQuery("days_ago")
	count, y := c.GetQuery("count")
	if !x && !y {
		c.HTML(200, "review.html", nil)
		return
	}
	words, err := his.Review(days, count)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("bad review request: %v", err))
		return
	}
	c.String(200, "%v", words)
}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.timeout.Stop() {
		select {
		case t := <-s.timeout.C:
			log.Debugf("drained from timer: %v", t)
		default:
		}
	}
	if *idleTimeout > 0 {
		s.timeout.Reset(*idleTimeout)
	}
	s.e.ServeHTTP(w, r)
}

func NewProxy() *proxy {
	r := httpserver.New(httpserver.Options{
		History:    his,
		EnableAuth: true,
		AuthSetup: func(r *gin.Engine) {
			store := cookie.NewStore([]byte("secret-key"))
			r.Use(sessions.Sessions("session", store))
			r.GET("/login", loginHandler)
			r.POST("/login", processLogin)
			r.GET("/auth", authMiddleware(), reviewHandler)
		},
		// On desktop: fall back to on-demand MDD extraction for anything not
		// already on disk in TmpDir (served by the static middleware below).
		ResourceHandler: httpserver.MddFileHandler,
	})

	// Also serve pre-dumped files (audio/images already on disk) from TmpDir.
	// This runs before NoRoute so cached files are served instantly.
	r.Use(static.Serve("/", static.LocalFile(util.TmpDir(), false)))

	return &proxy{e: r}
}

func ParseAddr(listen string) (network string, address string) {
	if listen == "auto" {
		return "auto", ""
	}
	if parts := strings.SplitN(listen, ";", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "tcp", listen
}
