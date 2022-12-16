package main

import (
	"html/template"

	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"
)

func markdownToHTML(md string) template.HTML {
	return template.HTML(
		bluemonday.UGCPolicy().SanitizeBytes(
			markdown.ToHTML([]byte(md), nil, nil),
		),
	)
}
