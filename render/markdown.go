package render

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

type MarkdownRender struct {
	Raw        string
	SourceType string
}

func (m *MarkdownRender) Render() string {
	var res string
	fd := strings.NewReader(m.Raw)
	if m.SourceType == LongmanEasy {
		res = ParseMDX(fd, "md")
	} else if m.SourceType == Longman5Online {
		res = ParseHTML(fd)
	} else {
		log.Warnf("undefined markdown render for %q, using general markdownify.", m.Raw)
		res = Markdownify(fd)
	}
	return res
}
