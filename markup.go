package main

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"
)

var reRef = regexp.MustCompile(`#(?P<Ref>([a-z]|_)+)\s*`)
var reRefIdIndex = reRef.SubexpIndex("Ref")

func markdownToHTML(references map[string]string, md string) template.HTML {
	// Resolve ('#') references
	foundReferences := map[string]string{}
	for _, match := range reRef.FindAllStringSubmatch(md, -1) {
		id := match[reRefIdIndex]
		if link, ok := references[id]; ok {
			foundReferences[id] = link
		}
	}
	html := string(bluemonday.UGCPolicy().SanitizeBytes(
		markdown.ToHTML([]byte(md), nil, nil),
	))
	// bluemonday.UGCPolicy() strips classes.
	// Resolving references after sanitization as a quick fix.
	for ref, link := range foundReferences {
		html = strings.ReplaceAll(html, "#"+ref, fmt.Sprintf(`<a href="%v" class="ref-link" >#%v</a>`, link, ref))
	}
	return template.HTML(html)
}
