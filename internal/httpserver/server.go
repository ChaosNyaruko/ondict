// Package httpserver contains the shared Gin HTTP server used by both the
// desktop (main) and mobile entry points. Callers configure behaviour via
// Options and get back a *gin.Engine ready to be Run on any net.Listener.
package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/history"
	"github.com/ChaosNyaruko/ondict/internal/tmpl"
	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/wordbank"
)

// Options configures the HTTP server.
type Options struct {
	// History is used to record queries. May be nil (disables recording).
	History *history.History

	// EnableAuth adds /login, /auth, and /review routes with session middleware.
	EnableAuth bool

	// AuthSetup is called after the gin.Engine is created so the caller can
	// attach session middleware and auth route handlers. Only called when
	// EnableAuth is true.
	AuthSetup func(r *gin.Engine)

	// Middleware is applied globally before any routes are registered.
	Middleware []gin.HandlerFunc

	// ResourceHandler is registered as NoRoute to serve static assets
	// (audio, images) on unmatched paths. If nil, no NoRoute is registered
	// and the caller is responsible for serving assets (e.g. via static middleware).
	ResourceHandler gin.HandlerFunc
}

// PageData is the template context shared across all HTML pages.
type PageData struct {
	Title      string
	Query      string
	Engine     string
	SearchMode string
	Error      string
	EntryHTML  template.HTML
	Results    []DefinitionMatchView
	Words      []wordbank.Word
	InWordBank bool
}

// DefinitionMatchView is a single definition search result for the template.
type DefinitionMatchView struct {
	Word        string
	Src         string
	SnippetHTML template.HTML
}

// New builds and returns a configured *gin.Engine.
func New(opts Options) *gin.Engine {
	r := gin.Default()
	r.SetHTMLTemplate(tmpl.Must())

	for _, mw := range opts.Middleware {
		r.Use(mw)
	}

	if opts.EnableAuth && opts.AuthSetup != nil {
		opts.AuthSetup(r)
	}

	r.GET("/", index)
	r.GET("/dict", queryWord(opts.History))
	r.GET("/search", searchHandler(opts.History))
	r.GET("/words", wordsHandler)
	r.POST("/words/add", addWordHandler)
	r.POST("/words/remove", removeWordHandler)
	r.GET("/complete", completeHandler)

	if opts.ResourceHandler != nil {
		r.NoRoute(opts.ResourceHandler)
	}

	return r
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

func index(c *gin.Context) {
	c.HTML(http.StatusOK, "portal.html", PageData{
		Title:      "Ondict",
		Engine:     "mdx",
		SearchMode: "headword",
	})
}

func queryWord(his *history.History) gin.HandlerFunc {
	return func(c *gin.Context) {
		word, _ := c.GetQuery("query")
		e, _ := c.GetQuery("engine")
		f, _ := c.GetQuery("format")
		rec, _ := c.GetQuery("record")

		if f != "html" {
			c.String(http.StatusOK, queryMDX(word, e, f, his, rec != "0"))
			return
		}
		if e == "" {
			e = "mdx"
		}
		inWordBank, err := wordbank.Contains(word)
		if err != nil {
			log.Debugf("check word bank %q err: %v", word, err)
		}
		res := queryMDX(word, e, f, his, rec != "0")
		c.HTML(http.StatusOK, "dict.html", PageData{
			Title:      PageTitle(word),
			Query:      word,
			Engine:     e,
			SearchMode: "headword",
			EntryHTML:  template.HTML(res),
			InWordBank: inWordBank,
		})
	}
}

func searchHandler(his *history.History) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			c.String(http.StatusOK, queryMDX(queryText, engine, format, his, record != "0"))
			return
		}

		if format != "html" {
			c.String(http.StatusOK, QueryDefinition(queryText))
			return
		}

		data := PageData{
			Title:      PageTitle(queryText),
			Query:      queryText,
			Engine:     engine,
			SearchMode: "definition",
		}
		if record != "0" && queryText != "" && his != nil {
			if err := his.Append(queryText); err != nil {
				log.Debugf("record definition search %q err: %v", queryText, err)
			}
		}
		if queryText != "" {
			matches, err := sources.SearchDefinitions(queryText, 20)
			if err != nil {
				data.Error = err.Error()
			} else {
				data.Results = make([]DefinitionMatchView, 0, len(matches))
				for _, match := range matches {
					data.Results = append(data.Results, DefinitionMatchView{
						Word:        match.Word,
						Src:         match.Src,
						SnippetHTML: template.HTML(match.Snippet),
					})
				}
			}
		}
		c.HTML(http.StatusOK, "search.html", data)
	}
}

func wordsHandler(c *gin.Context) {
	words, err := wordbank.List()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("list word bank: %v", err))
		return
	}
	c.HTML(http.StatusOK, "words.html", PageData{
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
	c.Redirect(http.StatusSeeOther, SafeNext(c.PostForm("next")))
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
	c.Redirect(http.StatusSeeOther, SafeNext(c.PostForm("next")))
}

func completeHandler(c *gin.Context) {
	prefix, ok := c.GetQuery("prefix")
	if !ok {
		c.String(200, "prefix empty")
		return
	}
	mode := sources.ParseCompletionMode(c.DefaultQuery("mode", "prefix"))
	suggestions := sources.Complete(prefix, mode, 10)
	res, err := json.Marshal(suggestions)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("marshal error"))
		return
	}
	c.Data(200, "application/json", res)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func queryMDX(word, engine, format string, his *history.History, record bool) string {
	if record && his != nil {
		if err := his.Append(word); err != nil {
			log.Debugf("record %v err: %v", word, err)
		}
	}
	if engine == "mdx" || engine == "" {
		return sources.QueryMDX(word, format)
	}
	return sources.GetFromLDOCE(word)
}

// QueryDefinition is exported so main.go can call it for the CLI one-shot mode.
func QueryDefinition(word string) string {
	matches, err := sources.SearchDefinitions(word, 20)
	if err != nil {
		return fmt.Sprintf("definition search error: %v", err)
	}
	if len(matches) == 0 {
		return fmt.Sprintf("no definition matches for %q", word)
	}
	lines := make([]string, 0, len(matches))
	for i, match := range matches {
		snippet := strings.ReplaceAll(match.Snippet, "<mark>", "")
		snippet = strings.ReplaceAll(snippet, "</mark>", "")
		if match.Src != "" {
			lines = append(lines, fmt.Sprintf("%d. %s (%s)\n   %s", i+1, match.Word, match.Src, snippet))
		} else {
			lines = append(lines, fmt.Sprintf("%d. %s\n   %s", i+1, match.Word, snippet))
		}
	}
	return strings.Join(lines, "\n")
}

// MddFileHandler serves a single MDD resource (audio/image) on demand.
// Register it as NoRoute to handle <img src="/..."> and <audio src="/...">.
func MddFileHandler(c *gin.Context) {
	filename := strings.TrimPrefix(c.Request.URL.Path, "/")
	data := sources.GetMDDFile(filename)
	if data == nil {
		c.Status(http.StatusNotFound)
		return
	}
	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(filename, ".mp3"):
		contentType = "audio/mpeg"
	case strings.HasSuffix(filename, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(filename, ".jpg"), strings.HasSuffix(filename, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(filename, ".css"):
		contentType = "text/css"
	}
	c.Data(http.StatusOK, contentType, data)
}

// PageTitle formats the browser tab title.
func PageTitle(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "Ondict"
	}
	return query + " - Ondict"
}

// SafeNext validates a redirect target is a local path.
func SafeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/words"
}
