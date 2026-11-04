package esign

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Line is one line of a generated text page.
type Line struct {
	Text string
	Size float64
	Bold bool
	// Muted draws the line in grey.
	Muted bool
	// Gap adds space above the line, in points.
	Gap float64
}

const (
	pageWidth  = 595.28 // A4
	pageHeight = 841.89
	margin     = 56.0
)

// TextPDF lays out lines on A4 pages with the standard Helvetica fonts and
// returns a complete PDF. Long lines wrap; pages break as needed.
func TextPDF(lines []Line) []byte {
	var pages []string
	var page strings.Builder
	y := pageHeight - margin
	flush := func() {
		pages = append(pages, page.String())
		page.Reset()
		y = pageHeight - margin
	}
	for _, line := range lines {
		size := line.Size
		if size == 0 {
			size = 10
		}
		for i, text := range wrap(line.Text, size, pageWidth-2*margin) {
			leading := size * 1.45
			if i == 0 {
				leading += line.Gap
			}
			if y-leading < margin {
				flush()
			}
			y -= leading
			font := "F1"
			if line.Bold {
				font = "F2"
			}
			gray := "0 g"
			if line.Muted {
				gray = "0.42 g"
			}
			fmt.Fprintf(&page, "BT %s /%s %.1f Tf %.2f %.2f Td (%s) Tj ET\n", gray, font, size, margin, y, escape(text))
		}
	}
	if page.Len() > 0 || len(pages) == 0 {
		flush()
	}
	return assemble(pages)
}

// wrap breaks text to fit width using approximate Helvetica metrics.
func wrap(text string, size, width float64) []string {
	maxChars := int(width / (size * 0.5))
	if maxChars < 10 {
		maxChars = 10
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	current := ""
	for _, word := range words {
		for len(word) > maxChars {
			if current != "" {
				out = append(out, current)
				current = ""
			}
			out = append(out, word[:maxChars])
			word = word[maxChars:]
		}
		switch {
		case current == "":
			current = word
		case len(current)+1+len(word) <= maxChars:
			current += " " + word
		default:
			out = append(out, current)
			current = word
		}
	}
	return append(out, current)
}

// escape encodes text as WinAnsi for a PDF literal string.
func escape(text string) string {
	encoder := charmap.Windows1252.NewEncoder()
	var out strings.Builder
	for _, r := range text {
		b, err := encoder.Bytes([]byte(string(r)))
		if err != nil || len(b) != 1 {
			out.WriteByte('?')
			continue
		}
		switch c := b[0]; c {
		case '(', ')', '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		default:
			if c < 32 {
				out.WriteByte(' ')
			} else {
				out.WriteByte(c)
			}
		}
	}
	return out.String()
}

func assemble(contents []string) []byte {
	var buf bytes.Buffer
	offsets := []int{}
	object := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	buf.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	// 1 catalog, 2 pages, 3-4 fonts, then a page and its content per page.
	kids := make([]string, len(contents))
	for i := range contents {
		kids[i] = fmt.Sprintf("%d 0 R", 5+i*2)
	}
	object("<< /Type /Catalog /Pages 2 0 R >>")
	object(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(contents)))
	object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, content := range contents {
		object(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", pageWidth, pageHeight, 6+i*2))
		object(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content))
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}
