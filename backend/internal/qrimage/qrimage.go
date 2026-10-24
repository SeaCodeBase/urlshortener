// Package qrimage renders plain (undecorated) QR code images as PNG or SVG.
package qrimage

import (
	"errors"
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	MinSize     = 64
	MaxSize     = 2048
	DefaultSize = 512

	defaultForeground = "#000000"
	defaultBackground = "#FFFFFF"
	defaultLevel      = "H"
)

var (
	ErrInvalidColor = errors.New("color must be #RRGGBB")
	ErrInvalidLevel = errors.New("error correction must be L, M, Q or H")
	ErrInvalidSize  = fmt.Errorf("size must be between %d and %d", MinSize, MaxSize)
)

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Options control rendering; zero values select the defaults (512 px, black
// on white, level H).
type Options struct {
	Size       int
	Foreground string
	Background string
	Level      string
}

type resolved struct {
	size   int
	fg, bg string
	level  qrcode.RecoveryLevel
}

func resolve(o Options) (resolved, error) {
	r := resolved{size: o.Size, fg: o.Foreground, bg: o.Background}
	if r.size == 0 {
		r.size = DefaultSize
	}
	if r.size < MinSize || r.size > MaxSize {
		return r, ErrInvalidSize
	}
	if r.fg == "" {
		r.fg = defaultForeground
	}
	if r.bg == "" {
		r.bg = defaultBackground
	}
	if !hexColor.MatchString(r.fg) || !hexColor.MatchString(r.bg) {
		return r, ErrInvalidColor
	}
	level := o.Level
	if level == "" {
		level = defaultLevel
	}
	switch level {
	case "L":
		r.level = qrcode.Low
	case "M":
		r.level = qrcode.Medium
	case "Q":
		r.level = qrcode.High
	case "H":
		r.level = qrcode.Highest
	default:
		return r, ErrInvalidLevel
	}
	return r, nil
}

func parseHex(s string) color.NRGBA {
	v, _ := strconv.ParseUint(s[1:], 16, 32)
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

// PNG renders content as a size×size PNG.
func PNG(content string, o Options) ([]byte, error) {
	r, err := resolve(o)
	if err != nil {
		return nil, err
	}
	q, err := qrcode.New(content, r.level)
	if err != nil {
		return nil, err
	}
	q.ForegroundColor = parseHex(r.fg)
	q.BackgroundColor = parseHex(r.bg)
	return q.PNG(r.size)
}

// SVG renders content as an SVG with one background rect and one path of
// module squares; the viewBox is in modules (quiet zone included).
func SVG(content string, o Options) ([]byte, error) {
	r, err := resolve(o)
	if err != nil {
		return nil, err
	}
	q, err := qrcode.New(content, r.level)
	if err != nil {
		return nil, err
	}
	bitmap := q.Bitmap()
	n := len(bitmap)

	var path strings.Builder
	for y, row := range bitmap {
		for x, on := range row {
			if on {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x, y)
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges">`,
		r.size, r.size, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, n, n, r.bg)
	fmt.Fprintf(&b, `<path fill="%s" d="%s"/>`, r.fg, path.String())
	b.WriteString(`</svg>`)
	return []byte(b.String()), nil
}
