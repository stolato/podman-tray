package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

const iconSize = 44

// Status dot in the bottom-right corner, in icon pixels (y down). The pod is
// cleared up to dotGap around it so the dot stays readable.
const (
	dotX   = 37.0
	dotY   = 34.0
	dotR   = 5.5
	dotGap = 8.0
)

// trayIcon draws a pea pod (capsule outline with three peas) as a black mask PNG,
// with room cut out for the status dot when withDot is set.
func trayIcon(withDot bool) []byte {
	return encodePNG(podMask(withDot))
}

// podMask is the pod shape as black with anti-aliased alpha.
func podMask(withDot bool) *image.NRGBA {
	const size = iconSize
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	const (
		cy        = 22.0
		left      = 13.0
		right     = 31.0
		podRadius = 11.0
		stroke    = 1.4
		peaRadius = 4.2
	)
	peas := []float64{left, (left + right) / 2, right}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5

			// Distance to the capsule's center segment.
			cx := math.Max(left, math.Min(right, px))
			d := math.Hypot(px-cx, py-cy)
			a := coverage(stroke - math.Abs(d-podRadius))

			for _, pcx := range peas {
				a = math.Max(a, coverage(peaRadius-math.Hypot(px-pcx, py-cy)))
			}
			if withDot {
				a = math.Min(a, coverage(math.Hypot(px-dotX, py-dotY)-dotGap))
			}
			if a > 0 {
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(a * 255)})
			}
		}
	}

	return img
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// coverage turns a signed distance (positive = inside) into an anti-aliased alpha.
func coverage(d float64) float64 {
	return math.Max(0, math.Min(1, d+0.5))
}
