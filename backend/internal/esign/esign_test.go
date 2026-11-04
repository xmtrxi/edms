package esign

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/digitorus/pdfsign/verify"
)

func signatureImage(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 300, 100))
	for x := 10; x < 290; x++ {
		img.Set(x, 50+(x%40)-20, color.NRGBA{A: 255})
		img.Set(x, 51+(x%40)-20, color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStampAppendSeal(t *testing.T) {
	ctx := context.Background()
	src := TextPDF([]Line{{Text: "Service agreement", Size: 18, Bold: true}, {Text: "Signed by:", Gap: 400}})
	if n, err := PageCount(ctx, src); err != nil || n != 1 {
		t.Fatalf("page count = %d, %v", n, err)
	}
	stamped, err := Stamp(ctx, src, []Mark{
		{Page: 1, X: 0.1, Y: 0.6, W: 0.3, H: 0.08, Image: signatureImage(t)},
		{Page: 1, X: 0.5, Y: 0.62, W: 0.3, H: 0.03, Text: "Jane Wanjiru — 30 Sep 2026"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Stamp(ctx, src, []Mark{{Page: 2, X: 0, Y: 0, W: 0.1, H: 0.1, Text: "x"}}); err != ErrInvalidMark {
		t.Fatalf("out-of-range page: %v", err)
	}
	full, err := Append(ctx, stamped, TextPDF([]Line{{Text: "Certificate of completion", Size: 16, Bold: true}}))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := PageCount(ctx, full); n != 2 {
		t.Fatalf("pages after append = %d", n)
	}
	certPEM, keyPEM, err := SelfSigned("Test EDMS")
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := NewSealer(certPEM, keyPEM, "")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealer.Seal(full, SealInfo{Name: "Test EDMS", Reason: "Signed"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := verify.VerifyFile(bytesFile(t, sealed))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Signers) != 1 || !result.Signers[0].ValidSignature {
		t.Fatalf("seal did not verify: %+v", result)
	}
	if dir := os.Getenv("ESIGN_TEST_OUT"); dir != "" {
		_ = os.WriteFile(dir+"/sealed.pdf", sealed, 0o644)
	}
}

func bytesFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	_, _ = f.Seek(0, 0)
	return f
}
