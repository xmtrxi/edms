package markdown

import (
	"regexp"
	"strings"
)

var (
	embeddedMarkdownImage = regexp.MustCompile(`(?im)!\[[^\]\r\n]*\]\(\s*data:image/[^)\r\n]*(?:\)|$)`)
	embeddedHTMLImage     = regexp.MustCompile(`(?im)<img\b[^>\r\n]*\bsrc=["']data:image/[^>\r\n]*(?:>|$)`)
	excessBlankLines      = regexp.MustCompile(`\n{3,}`)
)

// WithoutEmbeddedDataImages removes inline image payloads while preserving
// normal Markdown images, links, and searchable document text.
func WithoutEmbeddedDataImages(value string) string {
	value = embeddedMarkdownImage.ReplaceAllString(value, "")
	value = embeddedHTMLImage.ReplaceAllString(value, "")
	value = excessBlankLines.ReplaceAllString(value, "\n\n")
	return strings.TrimSpace(value)
}
