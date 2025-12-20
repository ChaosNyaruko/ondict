package util

import (
	"fmt"
	"net/url"
	"strings"
)

func ReplaceLINK(raw string) string {
	if link, ok := strings.CutPrefix(raw, "@@@LINK="); ok && string(link)[len(link)-1] == 0 {
		link = link[:len(link)-3] // ending with \r\n\x00
		raw = fmt.Sprintf(`
See <a class=Crossrefto href="/dict?query=%s&engine=mdx&format=html">%s</a> for more
</div>`,

			url.QueryEscape(link), link)
	}
	return raw
}
