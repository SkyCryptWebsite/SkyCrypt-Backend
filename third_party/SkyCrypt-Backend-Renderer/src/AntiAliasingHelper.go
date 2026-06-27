package src

import (
	"image"
	"image/color"
	"math"
	"sync"
)

func luma(c color.RGBA) float32 {
	return (float32(c.R)*0.299 + float32(c.G)*0.587 + float32(c.B)*0.114) * (float32(c.A) / 255.0)
}

func sampleBilinear(img *image.RGBA, x, y float32) color.RGBA {
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()

	if width <= 0 || height <= 0 {
		return color.RGBA{}
	}
	if width == 1 || height == 1 {
		return img.RGBAAt(b.Min.X, b.Min.Y)
	}

	ix := int(math.Floor(float64(x)))
	iy := int(math.Floor(float64(y)))
	fx := x - float32(ix)
	fy := y - float32(iy)

	ix = clampInt(ix, 0, width-2)
	iy = clampInt(iy, 0, height-2)

	px := b.Min.X + ix
	py := b.Min.Y + iy

	c00 := img.RGBAAt(px, py)
	c10 := img.RGBAAt(px+1, py)
	c01 := img.RGBAAt(px, py+1)
	c11 := img.RGBAAt(px+1, py+1)

	oneMinusFx := 1.0 - fx
	oneMinusFy := 1.0 - fy

	r := float32(c00.R)*oneMinusFx*oneMinusFy + float32(c10.R)*fx*oneMinusFy + float32(c01.R)*oneMinusFx*fy + float32(c11.R)*fx*fy
	g := float32(c00.G)*oneMinusFx*oneMinusFy + float32(c10.G)*fx*oneMinusFy + float32(c01.G)*oneMinusFx*fy + float32(c11.G)*fx*fy
	bl := float32(c00.B)*oneMinusFx*oneMinusFy + float32(c10.B)*fx*oneMinusFy + float32(c01.B)*oneMinusFx*fy + float32(c11.B)*fx*fy
	a := float32(c00.A)*oneMinusFx*oneMinusFy + float32(c10.A)*fx*oneMinusFy + float32(c01.A)*oneMinusFx*fy + float32(c11.A)*fx*fy

	return color.RGBA{
		R: clampByte(r),
		G: clampByte(g),
		B: clampByte(bl),
		A: clampByte(a),
	}
}

func ApplyFxaa(img *image.RGBA) {
	if img == nil {
		return
	}

	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()
	if width < 3 || height < 3 {
		return
	}

	tempImage := image.NewRGBA(b)
	copy(tempImage.Pix, img.Pix)

	const fxaaReduceMin float32 = 1.0 / 128.0
	const fxaaReduceMul float32 = 1.0 / 4.0
	const fxaaSpanMax float32 = 8.0

	var wg sync.WaitGroup
	for y := 1; y < height-1; y++ {
		y := y
		wg.Add(1)
		go func() {
			defer wg.Done()

			for x := 1; x < width-1; x++ {
				rgbNw := tempImage.RGBAAt(b.Min.X+x-1, b.Min.Y+y-1)
				rgbNe := tempImage.RGBAAt(b.Min.X+x+1, b.Min.Y+y-1)
				rgbSw := tempImage.RGBAAt(b.Min.X+x-1, b.Min.Y+y+1)
				rgbSe := tempImage.RGBAAt(b.Min.X+x+1, b.Min.Y+y+1)
				rgbM := tempImage.RGBAAt(b.Min.X+x, b.Min.Y+y)

				lumaNw := luma(rgbNw)
				lumaNe := luma(rgbNe)
				lumaSw := luma(rgbSw)
				lumaSe := luma(rgbSe)
				lumaM := luma(rgbM)

				lumaMin := minFloat32(lumaM, minFloat32(minFloat32(lumaNw, lumaNe), minFloat32(lumaSw, lumaSe)))
				lumaMax := maxFloat32(lumaM, maxFloat32(maxFloat32(lumaNw, lumaNe), maxFloat32(lumaSw, lumaSe)))

				contrast := lumaMax - lumaMin
				if contrast < maxFloat32(0.0156, lumaMax*0.0312) {
					continue
				}

				dirX := -((lumaNw + lumaNe) - (lumaSw + lumaSe))
				dirY := (lumaNw + lumaSw) - (lumaNe + lumaSe)

				dirReduce := maxFloat32((lumaNw+lumaNe+lumaSw+lumaSe)*(0.25*fxaaReduceMul), fxaaReduceMin)
				rcpDirMin := 1.0 / (minFloat32(absFloat32(dirX), absFloat32(dirY)) + dirReduce)

				dirX = clampFloat32(dirX*rcpDirMin, -fxaaSpanMax, fxaaSpanMax)
				dirY = clampFloat32(dirY*rcpDirMin, -fxaaSpanMax, fxaaSpanMax)

				sample1 := sampleBilinear(tempImage, float32(x)+dirX*(1.0/3.0-0.5), float32(y)+dirY*(1.0/3.0-0.5))
				sample2 := sampleBilinear(tempImage, float32(x)+dirX*(2.0/3.0-0.5), float32(y)+dirY*(2.0/3.0-0.5))

				r := (float32(sample1.R)*float32(sample1.A) + float32(sample2.R)*float32(sample2.A)) * 0.5
				g := (float32(sample1.G)*float32(sample1.A) + float32(sample2.G)*float32(sample2.A)) * 0.5
				bl := (float32(sample1.B)*float32(sample1.A) + float32(sample2.B)*float32(sample2.A)) * 0.5
				a := (float32(sample1.A) + float32(sample2.A)) * 0.5

				if a > 0 {
					img.SetRGBA(b.Min.X+x, b.Min.Y+y, color.RGBA{
						R: clampByte(r / a),
						G: clampByte(g / a),
						B: clampByte(bl / a),
						A: clampByte(a),
					})
				} else {
					img.SetRGBA(b.Min.X+x, b.Min.Y+y, color.RGBA{})
				}
			}
		}()
	}

	wg.Wait()
}

func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func clampFloat32(v, low, high float32) float32 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func clampByte(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
