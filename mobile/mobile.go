// Package mobile provides an entry point for gomobile to start the ondict
// HTTP server on Android. Only the dictionary query, search, autocomplete,
// word bank, and static file (audio/images) routes are included; auth and
// review routes are omitted for simplicity.
package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/internal/tmpl"
	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/util"
	"github.com/ChaosNyaruko/ondict/wordbank"
)

// mddAlreadyDumped returns true if MDD resources have been extracted before.
func mddAlreadyDumped(cacheDir string) bool {
	marker := filepath.Join(cacheDir, ".mdd_dumped")
	_, err := os.Stat(marker)
	return err == nil
}

// markMDDDumped writes a marker file so future launches skip the MDD dump.
func markMDDDumped(cacheDir string) {
	marker := filepath.Join(cacheDir, ".mdd_dumped")
	f, err := os.Create(marker)
	if err != nil {
		log.Warnf("could not write mdd dump marker: %v", err)
		return
	}
	f.Close()
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

// StartServer starts the ondict HTTP server on 127.0.0.1:<port>.
//
// configDir should be the app's private files directory
// (e.g. context.getFilesDir().getAbsolutePath() in Kotlin).
// Dictionary files (.mdx/.mdd) are expected under configDir/dicts/.
//
// cacheDir should be the app's cache directory
// (e.g. context.getCacheDir().getAbsolutePath() in Kotlin).
// Extracted MDD resources (audio, images) are written here.
//
// This function blocks; call it in a goroutine from the Android Activity.
func StartServer(configDir, cacheDir string, port int) {
	// Redirect logs to a file in cacheDir so we can inspect them on device
	logFile, err := os.OpenFile(
		filepath.Join(cacheDir, "ondict.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666,
	)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.SetLevel(log.DebugLevel)

	// Catch any panic so the goroutine doesn't silently die
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("StartServer panic: %v", r)
		}
	}()

	util.SetPaths(configDir, cacheDir)
	log.Infof("StartServer: configDir=%s cacheDir=%s port=%d", configDir, cacheDir, port)

	// Only dump MDD resources if not already done.
	// On mobile we use lazy on-demand extraction instead of bulk dump.
	// dumpMDD is always false; MDD is loaded lazily for GetFile() calls.
	gin.SetMode(gin.ReleaseMode)
	sources.G.Load(true /* iexact */, false /* dumpMDD */, true /* lazy */)

	r := gin.Default()

	r.SetHTMLTemplate(tmpl.Must())

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "portal.html", pageData{
			Title:      "Ondict",
			Engine:     "mdx",
			SearchMode: "headword",
		})
	})

	r.GET("/dict", queryWord)
	r.GET("/search", searchHandler)
	r.GET("/words", wordsHandler)
	r.POST("/words/add", addWordHandler)
	r.POST("/words/remove", removeWordHandler)
	r.GET("/complete", completeHandler)

	// Catch-all: serve any unmatched path from the MDD on demand.
	// This handles <img src="/snd_uk.png"> and <audio src="/GB_hello0205.mp3">
	// without requiring a full upfront MDD dump.
	r.NoRoute(mddFileHandler)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("mobile: listen %s: %v", addr, err)
	}
	log.Infof("mobile: ondict server listening on %s", addr)
	if err := r.RunListener(l); err != nil {
		log.Fatalf("mobile: server exited: %v", err)
	}
}

func queryWord(c *gin.Context) {
	word, _ := c.GetQuery("query")
	e, _ := c.GetQuery("engine")
	f, _ := c.GetQuery("format")

	if f != "html" {
		c.String(http.StatusOK, queryMDX(word, e, f))
		return
	}
	if e == "" {
		e = "mdx"
	}
	inWordBank, err := wordbank.Contains(word)
	if err != nil {
		log.Debugf("check word bank %q err: %v", word, err)
	}
	res := queryMDX(word, e, f)
	c.HTML(http.StatusOK, "dict.html", pageData{
		Title:      pageTitle(word),
		Query:      word,
		Engine:     e,
		SearchMode: "headword",
		EntryHTML:  template.HTML(res),
		InWordBank: inWordBank,
	})
}

func searchHandler(c *gin.Context) {
	queryText := strings.TrimSpace(c.Query("query"))
	mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "headword")))
	engine := c.DefaultQuery("engine", "mdx")
	format := c.DefaultQuery("format", "html")

	if mode == "" {
		mode = "headword"
	}
	if mode == "headword" {
		target := fmt.Sprintf("/dict?query=%s&engine=%s&format=%s",
			url.QueryEscape(queryText),
			url.QueryEscape(engine),
			url.QueryEscape(format),
		)
		if format == "html" {
			c.Redirect(http.StatusFound, target)
			return
		}
		c.String(http.StatusOK, queryMDX(queryText, engine, format))
		return
	}

	if format != "html" {
		c.String(http.StatusOK, queryDefinition(queryText))
		return
	}

	data := pageData{
		Title:      pageTitle(queryText),
		Query:      queryText,
		Engine:     engine,
		SearchMode: "definition",
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

func queryMDX(word, engine, format string) string {
	if engine == "mdx" || engine == "" {
		return sources.QueryMDX(word, format)
	}
	return sources.GetFromLDOCE(word)
}

func queryDefinition(word string) string {
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
		lines = append(lines, fmt.Sprintf("%d. %s\n   %s", i+1, match.Word, snippet))
	}
	return strings.Join(lines, "\n")
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

// mddFileHandler serves a single resource file (audio/image) from the MDD
// dictionary on demand, without requiring a full upfront dump.
// It is registered as a NoRoute handler so any unmatched path is tried
// against the MDD key index (e.g. /GB_hello0205.mp3, /snd_uk.png).
func mddFileHandler(c *gin.Context) {
	// Strip the leading slash to get the bare filename/path
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
