package render

import (
	"golang.org/x/net/html"
)

// RenderContext carries per-render metadata available to all SchemeHandlers.
type RenderContext struct {
	// SourceType identifies the dictionary type (e.g. LongmanEasy, Longman5Online).
	SourceType string

	// EntryFetcher, when set, is called by handlers that need to fetch another
	// entry's rendered HTML (e.g. ShowImageHandler fetching big_pic from a
	// cross-referenced word). The function receives a plain word (no fragment)
	// and returns the rendered HTML string.
	EntryFetcher func(word string) string
}

// NodeHandler rewrites a single DOM node in-place. It is called during the
// DFS walk for every element node. Implementations should inspect the node and
// its attributes and mutate as needed. Returning true means the node was
// handled and the walker should NOT recurse into its children; false means the
// walker should continue recursing normally.
//
// Note: returning false is the right choice when the handler mutates the node
// but children still need to be visited (e.g. sound:// turns <a> into <div>
// but the children <img> still need their src fixed).
type NodeHandler interface {
	// HandleNode is called for every element node during DFS.
	// Returns true if children should be skipped.
	HandleNode(n *html.Node, ctx RenderContext) (skipChildren bool)
}
