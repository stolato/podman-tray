//go:build !darwin

package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"runtime"

	"fyne.io/systray"
)

// applyTrayIcon sets a colored icon: panels and taskbars don't recolor it like the macOS
// menu bar does, and can be light or dark.
func applyTrayIcon(active bool) {
	icon := encodePNG(colorIcon(active))
	if runtime.GOOS == "windows" {
		icon = pngToICO(icon, iconSize)
	}
	systray.SetIcon(icon)
}

// colorIcon draws the pod in white over a dark halo, which reads on light and dark
// backgrounds alike, plus the green dot when active.
func colorIcon(active bool) *image.NRGBA {
	mask := podMask(active)
	const halo = 1.6 // px
	img := image.NewNRGBA(mask.Bounds())
	for y := range iconSize {
		for x := range iconSize {
			fill := float64(mask.NRGBAAt(x, y).A) / 255
			shadow := 0.0
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if d := math.Hypot(float64(dx), float64(dy)); d <= halo {
						shadow = math.Max(shadow, float64(alphaAt(mask, x+dx, y+dy))/255)
					}
				}
			}
			if active {
				d := math.Hypot(float64(x)+0.5-dotX, float64(y)+0.5-dotY)
				shadow = math.Max(shadow, coverage(dotR+halo-d))
				if dot := coverage(dotR - d); dot > 0 {
					img.SetNRGBA(x, y, over(color.NRGBA{52, 199, 89, uint8(dot * 255)}, shadow))
					continue
				}
			}
			img.SetNRGBA(x, y, over(color.NRGBA{255, 255, 255, uint8(fill * 255)}, shadow))
		}
	}
	return img
}

func alphaAt(img *image.NRGBA, x, y int) uint8 {
	if !(image.Point{x, y}.In(img.Bounds())) {
		return 0
	}
	return img.NRGBAAt(x, y).A
}

// over composites fg over a dark halo with alpha shadow (scaled to 60%).
func over(fg color.NRGBA, shadow float64) color.NRGBA {
	fa := float64(fg.A) / 255
	sa := 0.6 * shadow
	a := fa + sa*(1-fa)
	if a == 0 {
		return color.NRGBA{}
	}
	mix := func(c uint8) uint8 { return uint8(float64(c) * fa / a) } // halo is black
	return color.NRGBA{mix(fg.R), mix(fg.G), mix(fg.B), uint8(a * 255)}
}

// pngToICO wraps a square PNG in an .ico container (Vista+ reads PNG entries), which is
// what systray needs on Windows.
func pngToICO(png []byte, size int) []byte {
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	le(uint16(0))                               // reserved
	le(uint16(1))                               // type: icon
	le(uint16(1))                               // image count
	le([]uint8{uint8(size), uint8(size), 0, 0}) // width, height, palette, reserved
	le(uint16(1))                               // color planes
	le(uint16(32))                              // bits per pixel
	le(uint32(len(png)))                        // image size
	le(uint32(6 + 16))                          // image offset: header + one entry
	b.Write(png)
	return b.Bytes()
}
