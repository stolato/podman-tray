// genicon renders the app icon (a pea pod on a rounded square) into a macOS .iconset.
//
//	go run ./tools/genicon packaging/AppIcon.iconset
//	iconutil -c icns packaging/AppIcon.iconset -o packaging/AppIcon.icns
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

// Geometry is laid out on a 1024×1024 canvas, following Apple's icon grid
// (824pt rounded square with ~185pt corners, centered).
const (
	canvas     = 1024.0
	bgHalf     = 412.0
	bgRadius   = 185.0
	podLeft    = 352.0
	podRight   = 672.0
	podRadius  = 150.0
	podStroke  = 22.0
	peaRadius  = 82.0
	tiltDegree = 28.0
)

var (
	bgTop     = rgb(0x9b, 0x5d, 0xe5)
	bgBottom  = rgb(0x4b, 0x23, 0x8a)
	podFill   = rgb(0x4f, 0x9d, 0x3a)
	podEdge   = rgb(0x2f, 0x6b, 0x22)
	peaFill   = rgb(0xa8, 0xe0, 0x63)
	peaShadow = rgb(0x78, 0xb8, 0x45)
	highlight = rgb(0xf2, 0xff, 0xd8)
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("uso: genicon <dir.iconset>")
	}
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, size := range []int{16, 32, 128, 256, 512} {
		write(filepath.Join(dir, fmt.Sprintf("icon_%dx%d.png", size, size)), size)
		write(filepath.Join(dir, fmt.Sprintf("icon_%dx%d@2x.png", size, size)), size*2)
	}
}

func write(path string, size int) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, render(size)); err != nil {
		log.Fatal(err)
	}
}

func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := canvas / float64(size)
	// One output pixel spans `scale` canvas units; used to anti-alias edges at any size.
	aa := func(d float64) float64 { return coverage(d / scale) }

	sin, cos := math.Sincos(-tiltDegree * math.Pi / 180)
	peas := []float64{podLeft, (podLeft + podRight) / 2, podRight}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := (float64(x)+0.5)*scale - canvas/2
			py := (float64(y)+0.5)*scale - canvas/2

			// Background: rounded square with a vertical gradient.
			bgA := aa(-roundedRect(px, py, bgHalf, bgRadius))
			if bgA == 0 {
				continue
			}
			c := mix(bgTop, bgBottom, (py+bgHalf)/(2*bgHalf))

			// Pod space: rotate around the center so the pod sits diagonally. Offsets for the
			// shadow and highlights are applied before rotating, so light comes from the top-left.
			toPod := func(dx, dy float64) (float64, float64) {
				x, y := px+dx, py+dy
				return x*cos + y*sin + canvas/2, -x*sin + y*cos
			}
			qx, qy := toPod(0, 0)

			// Soft drop shadow under the pod.
			sx, sy := toPod(0, -28)
			sd := capsule(sx, sy, podLeft, podRight, podRadius)
			c = mix(c, rgb(0x1c, 0x0b, 0x38), 0.35*smooth(-sd/60))

			pd := capsule(qx, qy, podLeft, podRight, podRadius)
			c = mix(c, podEdge, aa(-pd))
			c = mix(c, podFill, aa(-(pd + podStroke)))

			for _, cx := range peas {
				d := math.Hypot(qx-cx, qy) - peaRadius
				c = mix(c, peaShadow, aa(-d))
				fx, fy := toPod(10, 10)
				c = mix(c, peaFill, aa(-(math.Hypot(fx-cx, fy) - peaRadius + 12)))
				hx, hy := toPod(30, 30)
				c = mix(c, highlight, 0.85*aa(-(math.Hypot(hx-cx, hy)-18)))
			}

			c.A = uint8(bgA * 255)
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// roundedRect is the signed distance to a centered square of half-size h with corner radius r.
func roundedRect(x, y, h, r float64) float64 {
	qx, qy := math.Abs(x)-h+r, math.Abs(y)-h+r
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return outside + math.Min(math.Max(qx, qy), 0) - r
}

// capsule is the signed distance to a horizontal capsule on y=0 between x=left and x=right.
func capsule(x, y, left, right, r float64) float64 {
	cx := math.Max(left, math.Min(right, x))
	return math.Hypot(x-cx, y) - r
}

// coverage turns a signed distance in pixels (positive = inside) into an anti-aliased alpha.
func coverage(d float64) float64 {
	return math.Max(0, math.Min(1, d+0.5))
}

func smooth(t float64) float64 {
	t = math.Max(0, math.Min(1, t))
	return t * t * (3 - 2*t)
}

func rgb(r, g, b uint8) color.NRGBA { return color.NRGBA{r, g, b, 255} }

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	t = math.Max(0, math.Min(1, t))
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}
