// Package editor connects documents to ONLYOFFICE Docs for editing in the
// browser, and creates new blank Office files.
package editor

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Blank kinds.
const (
	KindDocument     = "document"
	KindSpreadsheet  = "spreadsheet"
	KindPresentation = "presentation"
)

// BlankFile describes a new empty file.
type BlankFile struct {
	Extension string
	MimeType  string
	Body      []byte
}

var ErrUnknownKind = errors.New("choose a document, spreadsheet or presentation")

// Blank builds an empty Office file. Each one carries a unique identifier,
// so two blank files are never treated as duplicates.
func Blank(kind, title, author string) (*BlankFile, error) {
	var parts map[string]string
	var file BlankFile
	switch kind {
	case KindDocument:
		parts, file = docxParts(), BlankFile{Extension: ".docx", MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}
	case KindSpreadsheet:
		parts, file = xlsxParts(), BlankFile{Extension: ".xlsx", MimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}
	case KindPresentation:
		parts, file = pptxParts(), BlankFile{Extension: ".pptx", MimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation"}
	default:
		return nil, ErrUnknownKind
	}
	parts["docProps/core.xml"] = coreProperties(title, author)
	body, err := pack(parts)
	if err != nil {
		return nil, err
	}
	file.Body = body
	return &file, nil
}

// pack writes the parts in a stable order, [Content_Types].xml first.
func pack(parts map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	names := []string{"[Content_Types].xml", "_rels/.rels"}
	for name := range parts {
		if name != names[0] && name != names[1] {
			names = append(names, name)
		}
	}
	for _, name := range names {
		w, err := archive.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(xml.Header + parts[name])); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func escapeXML(value string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func coreProperties(title, author string) string {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>%s</dc:title><dc:creator>%s</dc:creator><dc:identifier>%s</dc:identifier><dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified></cp:coreProperties>`,
		escapeXML(title), escapeXML(author), hex.EncodeToString(id), now, now)
}

const (
	nsRels      = "http://schemas.openxmlformats.org/package/2006/relationships"
	relOffice   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relCore     = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	typeRels    = "application/vnd.openxmlformats-package.relationships+xml"
	typeCore    = "application/vnd.openxmlformats-package.core-properties+xml"
	nsR         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	relBase     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	contentBase = "application/vnd.openxmlformats-officedocument."
)

func contentTypes(overrides map[string]string) string {
	var b strings.Builder
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="` + typeRels + `"/><Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/docProps/core.xml" ContentType="` + typeCore + `"/>`)
	for part, kind := range overrides {
		b.WriteString(`<Override PartName="` + part + `" ContentType="` + kind + `"/>`)
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func rootRels(main string) string {
	return `<Relationships xmlns="` + nsRels + `"><Relationship Id="rId1" Type="` + relOffice + `" Target="` + main + `"/><Relationship Id="rId2" Type="` + relCore + `" Target="docProps/core.xml"/></Relationships>`
}

func docxParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": contentTypes(map[string]string{"/word/document.xml": contentBase + "wordprocessingml.document.main+xml"}),
		"_rels/.rels":         rootRels("word/document.xml"),
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/>` +
			`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr></w:body></w:document>`,
	}
}

func xlsxParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": contentTypes(map[string]string{
			"/xl/workbook.xml":          contentBase + "spreadsheetml.sheet.main+xml",
			"/xl/worksheets/sheet1.xml": contentBase + "spreadsheetml.worksheet+xml",
		}),
		"_rels/.rels":                rootRels("xl/workbook.xml"),
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="` + nsR + `"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="` + nsRels + `"><Relationship Id="rId1" Type="` + relBase + `worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`,
	}
}

const presentationNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="` + nsR + `" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

const emptyTree = `<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree>`

func pptxParts() map[string]string {
	colors := []string{"2563EB", "DB2777", "059669", "D97706", "7C3AED", "0891B2"}
	var accents strings.Builder
	for i, c := range colors {
		fmt.Fprintf(&accents, `<a:accent%d><a:srgbClr val="%s"/></a:accent%d>`, i+1, c, i+1)
	}
	fill := `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
	line := `<a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`
	effect := `<a:effectStyle><a:effectLst/></a:effectStyle>`
	font := `<a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/>`
	theme := `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Lumen"><a:themeElements>` +
		`<a:clrScheme name="Lumen"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
		`<a:dk2><a:srgbClr val="1F2937"/></a:dk2><a:lt2><a:srgbClr val="F3F4F6"/></a:lt2>` + accents.String() +
		`<a:hlink><a:srgbClr val="2563EB"/></a:hlink><a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink></a:clrScheme>` +
		`<a:fontScheme name="Lumen"><a:majorFont>` + font + `</a:majorFont><a:minorFont>` + font + `</a:minorFont></a:fontScheme>` +
		`<a:fmtScheme name="Lumen"><a:fillStyleLst>` + strings.Repeat(fill, 3) + `</a:fillStyleLst><a:lnStyleLst>` + strings.Repeat(line, 3) +
		`</a:lnStyleLst><a:effectStyleLst>` + strings.Repeat(effect, 3) + `</a:effectStyleLst><a:bgFillStyleLst>` + strings.Repeat(fill, 3) +
		`</a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`
	rel := func(id, kind, target string) string {
		return `<Relationship Id="` + id + `" Type="` + relBase + kind + `" Target="` + target + `"/>`
	}
	rels := func(items ...string) string {
		return `<Relationships xmlns="` + nsRels + `">` + strings.Join(items, "") + `</Relationships>`
	}
	return map[string]string{
		"[Content_Types].xml": contentTypes(map[string]string{
			"/ppt/presentation.xml":              contentBase + "presentationml.presentation.main+xml",
			"/ppt/slides/slide1.xml":             contentBase + "presentationml.slide+xml",
			"/ppt/slideLayouts/slideLayout1.xml": contentBase + "presentationml.slideLayout+xml",
			"/ppt/slideMasters/slideMaster1.xml": contentBase + "presentationml.slideMaster+xml",
			"/ppt/theme/theme1.xml":              contentBase + "theme+xml",
		}),
		"_rels/.rels": rootRels("ppt/presentation.xml"),
		"ppt/presentation.xml": `<p:presentation ` + presentationNS + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
			`<p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": rels(rel("rId1", "slideMaster", "slideMasters/slideMaster1.xml"), rel("rId2", "slide", "slides/slide1.xml"), rel("rId3", "theme", "theme/theme1.xml")),
		"ppt/slideMasters/slideMaster1.xml": `<p:sldMaster ` + presentationNS + `><p:cSld>` + emptyTree + `</p:cSld>` +
			`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
			`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": rels(rel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml"), rel("rId2", "theme", "../theme/theme1.xml")),
		"ppt/slideLayouts/slideLayout1.xml": `<p:sldLayout ` + presentationNS + ` type="blank" preserve="1"><p:cSld name="Blank">` + emptyTree +
			`</p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": rels(rel("rId1", "slideMaster", "../slideMasters/slideMaster1.xml")),
		"ppt/slides/slide1.xml":                        `<p:sld ` + presentationNS + `><p:cSld>` + emptyTree + `</p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             rels(rel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml")),
		"ppt/theme/theme1.xml":                         theme,
	}
}
