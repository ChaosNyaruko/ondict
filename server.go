package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/util"
	"github.com/ChaosNyaruko/ondict/wordbank"
)

type proxy struct {
	e       *gin.Engine
	timeout *time.Timer
}

type pageData struct {
	Title      string
	Query      string
	Engine     string
	SearchMode string
	Error      string
	EntryHTML  template.HTML
	Results    []definitionMatchView
	Words      []wordbank.Word
	InWordBank bool
}

type definitionMatchView struct {
	Word        string
	Src         string
	SnippetHTML template.HTML
}

func (p *proxy) Run(l net.Listener) error {
	log.Infof("proxy started!")
	return p.e.RunListener(l)
}

func review(c *gin.Context) {
	days, x := c.GetQuery("days_ago")
	count, y := c.GetQuery("count")
	if !x && !y {
		c.HTML(200, "review.html", nil)
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

	if f != "html" {
		c.String(http.StatusOK, query(word, e, f, r != "0"))
		return
	}

	if e == "" {
		e = "mdx"
	}
	inWordBank, err := wordbank.Contains(word)
	if err != nil {
		log.Debugf("check word bank %q err: %v", word, err)
	}
	res := query(word, e, f, r != "0")
	c.HTML(http.StatusOK, "dict.html", pageData{
		Title:      pageTitle(word),
		Query:      word,
		Engine:     e,
		SearchMode: "headword",
		EntryHTML:  template.HTML(res),
		InWordBank: inWordBank,
	})
}

func index(c *gin.Context) {
	c.HTML(http.StatusOK, "portal.html", pageData{
		Title:      "Ondict",
		Engine:     "mdx",
		SearchMode: "headword",
	})
}

func searchHandler(c *gin.Context) {
	queryText := strings.TrimSpace(c.Query("query"))
	mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "headword")))
	engine := c.DefaultQuery("engine", "mdx")
	format := c.DefaultQuery("format", "html")
	record := c.Query("record")

	if mode == "" {
		mode = "headword"
	}
	if mode == "headword" {
		target := fmt.Sprintf("/dict?query=%s&engine=%s&format=%s&record=%s",
			url.QueryEscape(queryText),
			url.QueryEscape(engine),
			url.QueryEscape(format),
			url.QueryEscape(record),
		)
		if format == "html" {
			c.Redirect(http.StatusFound, target)
			return
		}
		c.String(http.StatusOK, query(queryText, engine, format, record != "0"))
		return
	}

	if format != "html" {
		c.String(http.StatusOK, queryDefinition(queryText, format, record != "0"))
		return
	}

	data := pageData{
		Title:      pageTitle(queryText),
		Query:      queryText,
		Engine:     engine,
		SearchMode: "definition",
	}
	if record != "0" && queryText != "" {
		if err := his.Append(queryText); err != nil {
			log.Debugf("record definition search %q err: %v", queryText, err)
		}
	}
	if queryText != "" {
		matches, err := sources.SearchDefinitions(queryText, 20)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Results = make([]definitionMatchView, 0, len(matches))
			for _, match := range matches {
				data.Results = append(data.Results, definitionMatchView{
					Word:        match.Word,
					Src:         match.Src,
					SnippetHTML: template.HTML(match.Snippet),
				})
			}
		}
	}
	c.HTML(http.StatusOK, "search.html", data)
}

func wordsHandler(c *gin.Context) {
	words, err := wordbank.List()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("list word bank: %v", err))
		return
	}
	c.HTML(http.StatusOK, "words.html", pageData{
		Title:      "Word Bank - Ondict",
		Engine:     "mdx",
		SearchMode: "headword",
		Words:      words,
	})
}

func addWordHandler(c *gin.Context) {
	word := c.PostForm("word")
	if err := wordbank.Add(word); err != nil {
		if errors.Is(err, wordbank.ErrEmptyWord) {
			c.AbortWithStatusJSON(http.StatusBadRequest, "word is empty")
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("add word: %v", err))
		return
	}
	c.Redirect(http.StatusSeeOther, safeNext(c.PostForm("next")))
}

func removeWordHandler(c *gin.Context) {
	word := c.PostForm("word")
	if err := wordbank.Remove(word); err != nil {
		if errors.Is(err, wordbank.ErrEmptyWord) {
			c.AbortWithStatusJSON(http.StatusBadRequest, "word is empty")
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("remove word: %v", err))
		return
	}
	c.Redirect(http.StatusSeeOther, safeNext(c.PostForm("next")))
}

//go:embed templates/*
var templateFS embed.FS

func NewProxy() *proxy {
	r := gin.Default()

	// 从嵌入的文件系统加载模板
	tpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tpl)
	// Set up cookie-based sessions
	store := cookie.NewStore([]byte("secret-key"))
	r.Use(sessions.Sessions("session", store))

	r.GET("/login", loginHandler)
	r.POST("/login", processLogin)
	r.GET("/auth", authMiddleware(), review)

	r.GET("/", index)
	r.Use(static.Serve("/", static.LocalFile(util.TmpDir(), false)))
	r.GET("/dict", queryWord)
	r.GET("/search", searchHandler)
	r.GET("/words", wordsHandler)
	r.POST("/words/add", addWordHandler)
	r.POST("/words/remove", removeWordHandler)
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
	mode := sources.ParseCompletionMode(c.DefaultQuery("mode", "prefix"))
	suggestions := sources.Complete(prefix, mode, 10)

	res, err := json.Marshal(suggestions)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("unmarshal error"))
		return
	}
	c.Data(200, "application/json", res)
}

func pageTitle(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "Ondict"
	}
	return query + " - Ondict"
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/words"
}
