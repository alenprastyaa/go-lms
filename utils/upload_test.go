package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"regexp"
	"strings"
	"testing"
)

func TestMaybeCompressUpload_EmptyContent(t *testing.T) {
	out, ct := maybeCompressUpload(nil, "")
	if len(out) != 0 {
		t.Fatalf("expected empty output")
	}
	if ct != "application/octet-stream" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestMaybeCompressUpload_JPEGTypeResolution(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * y), G: 120, B: 200, A: 255})
		}
	}
	var src bytes.Buffer
	if err := jpeg.Encode(&src, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	out, ct := maybeCompressUpload(src.Bytes(), "image/jpeg")
	if ct != "image/jpeg" {
		t.Fatalf("content-type = %q", ct)
	}
	if len(out) == 0 {
		t.Fatalf("compressed bytes should not be empty")
	}
}

func TestBuildObjectKey(t *testing.T) {
	key := buildObjectKey(" My File @2026!!.JPG ")
	if strings.TrimSpace(key) == "" {
		t.Fatalf("key should not be empty")
	}
	matched, _ := regexp.MatchString(`^\d+-my-file-+2026-+\.jpg$`, key)
	if !matched {
		t.Fatalf("unexpected key format: %q", key)
	}
}
