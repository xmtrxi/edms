// Package esign stamps signatures and field values onto PDFs, appends a
// certificate of completion and seals the result with a digital signature.
package esign

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decode JPEG signature images
	_ "image/png"  // decode PNG signature images
	"io"
	"math"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Mark is a value placed on a page. Position and size are fractions of the
// page (0..1) measured from the top-left corner, as the browser sees it.
type Mark struct {
	Page int // 1-based
	X, Y float64
	W, H float64
	// Image is a PNG or JPEG drawn to fit the box; otherwise Text is written.
	Image []byte
	Text  string
}

var ErrInvalidMark = errors.New("a field is outside the page")

// config never touches the filesystem; pdfcpu would otherwise create a
// configuration directory in $HOME.
func config() *model.Configuration {
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	return conf
}

// Stamp draws marks onto the PDF in src and returns the new PDF.
func Stamp(ctx context.Context, src []byte, marks []Mark) ([]byte, error) {
	if len(marks) == 0 {
		return src, nil
	}
	dims, err := api.PageDims(ctx, bytes.NewReader(src), config())
	if err != nil {
		return nil, fmt.Errorf("read page sizes: %w", err)
	}
	byPage := map[int][]*model.Watermark{}
	for _, mark := range marks {
		if mark.Page < 1 || mark.Page > len(dims) || mark.W <= 0 || mark.H <= 0 ||
			mark.X < 0 || mark.Y < 0 || mark.X+mark.W > 1.0001 || mark.Y+mark.H > 1.0001 {
			return nil, ErrInvalidMark
		}
		page := dims[mark.Page-1]
		box := rect{
			x: mark.X * page.Width, y: (1 - mark.Y - mark.H) * page.Height,
			w: mark.W * page.Width, h: mark.H * page.Height,
		}
		var wm *model.Watermark
		switch {
		case len(mark.Image) > 0:
			wm, err = imageMark(ctx, mark.Image, box)
		case strings.TrimSpace(mark.Text) != "":
			wm, err = textMark(ctx, mark.Text, box)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		byPage[mark.Page] = append(byPage[mark.Page], wm)
	}
	if len(byPage) == 0 {
		return src, nil
	}
	var out bytes.Buffer
	if err := api.AddWatermarksSliceMap(ctx, bytes.NewReader(src), &out, byPage, config()); err != nil {
		return nil, fmt.Errorf("stamp fields: %w", err)
	}
	return out.Bytes(), nil
}

type rect struct{ x, y, w, h float64 }

// imageMark fits the image inside box, keeping its aspect ratio, centred.
func imageMark(ctx context.Context, data []byte, box rect) (*model.Watermark, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("signature image: %w", err)
	}
	if cfg.Width == 0 || cfg.Height == 0 {
		return nil, errors.New("signature image is empty")
	}
	scale := math.Min(box.w/float64(cfg.Width), box.h/float64(cfg.Height))
	w, h := float64(cfg.Width)*scale, float64(cfg.Height)*scale
	x, y := box.x+(box.w-w)/2, box.y+(box.h-h)/2
	desc := fmt.Sprintf("pos:bl, off:%.2f %.2f, scale:%.5f abs, rot:0, op:1", x, y, scale)
	return api.ImageWatermarkForReader(ctx, bytes.NewReader(data), desc, true, false, types.POINTS)
}

// textMark writes text left-aligned and vertically centred in box, shrinking
// the font so it fits.
func textMark(ctx context.Context, text string, box rect) (*model.Watermark, error) {
	text = strings.Join(strings.Fields(text), " ")
	size := math.Min(box.h*0.62, 12)
	if width := float64(len([]rune(text))) * size * 0.52; width > box.w {
		size = math.Max(5, size*box.w/width)
	}
	size = math.Max(5, math.Floor(size))
	x, y := box.x+1, box.y+(box.h-size)/2+size*0.1
	desc := fmt.Sprintf("font:Helvetica, points:%.0f, pos:bl, off:%.2f %.2f, scale:1 abs, rot:0, op:1, fillc:#111111, margins:0", size, x, y)
	return api.TextWatermark(ctx, text, desc, true, false, types.POINTS, config())
}

// Append adds the pages of extra to the end of src.
func Append(ctx context.Context, src, extra []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := api.MergeRaw(ctx, []io.ReadSeeker{bytes.NewReader(src), bytes.NewReader(extra)}, &out, false, config()); err != nil {
		return nil, fmt.Errorf("append certificate: %w", err)
	}
	return out.Bytes(), nil
}

// PageCount returns the number of pages, which also checks the file is a
// PDF this package can work with.
func PageCount(ctx context.Context, src []byte) (int, error) {
	dims, err := api.PageDims(ctx, bytes.NewReader(src), config())
	if err != nil {
		return 0, err
	}
	return len(dims), nil
}
