package sources

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/ChaosNyaruko/ondict/render"
)

func QueryByURL(word string) string {
	start := time.Now()
	// queryURL := fmt.Sprintf("https://ldoceonline.com/dictionary/%s", word)
	queryURL := fmt.Sprintf("https://ldoceonline.com/search/english/direct/?q=%s", url.QueryEscape(word))
	// resp, err := http.Get(queryURL) // an unexpected EOF will occur
	// Refer to https://www.reddit.com/r/golang/comments/y971ye/unexpected_eof_from_http_request/ --> not working
	// https://bugz.pythonanywhere.com/golang/Unexpected-EOF-golang-http-client-error --> not working either
	// Maybe not my problem? It's work when I developed the first demo version. https://www.appsloveworld.com/go/2/golang-http-request-results-in-eof-errors-when-making-multiple-requests-successiv
	// I change my User-Agent to curl, it works then. 🥲
	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		queryURL,
		http.NoBody,
	)
	req.Close = true
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Accept-Encoding", "identity") // NOTE THIS LINE
	req.Header.Set("User-Agent", "curl/8.1.2")

	resp, err := client.Do(req)

	log.Printf("query %q cost: %v", queryURL, time.Since(start))
	if err != nil {
		log.Printf("Get url %v err: %v", queryURL, err)
		return fmt.Sprintf("ERROR: %v", err)
	}
	defer resp.Body.Close()
	return render.ParseHTML(resp.Body)
}

func GetFromLDOCE(word string) string {
	var res string
	mu.Lock()
	if ex, ok := history[word]; ok {
		log.Printf("cache hit!")
		res = ex
	} else {
		res = QueryByURL(word)
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
