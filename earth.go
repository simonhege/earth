package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"log"
	"math"
	"os"

	"github.com/llgcode/draw2d/draw2dimg"

	"github.com/xeonx/geom"
	"github.com/xeonx/geom/encoding/geojson"
	"github.com/xeonx/proj4"
)

const deg2Rad = math.Pi / 180.0
const rad2Deg = 180.0 / math.Pi

func main() {
	flag.Parse()

	proj.SetFinder([]string{"C:\\OSGeo4W64\\share\\proj"})

	//GeoJSON file to be animated is the first argument
	input := flag.Arg(0)
	if len(input) == 0 {
		log.Fatal("Invalid input file")
	}

	log.Print(input)

	file, err := os.Open(input) // For read access.
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	dec := geojson.NewDecoder(file)

	var c geojson.FeatureCollection
	if err := dec.DecodeCollection(&c); err != nil {
		log.Fatal(err)
	}

	if len(c.Features) == 0 {
		log.Fatal("No feature found")
	}

	projWGS84, err := proj.InitPlus("+init=epsg:4326")
	if err != nil {
		log.Fatal(err)
	}

	imgSize := 192.
	r := image.Rect(0, 0, int(imgSize), int(imgSize))

	var images []*image.Paletted
	colorBackground := color.RGBA{0xff, 0xff, 0xff, 0x00}
	colorOcean := color.RGBA{0x00, 0x66, 0xff, 0xff}
	colorBlack := color.RGBA{0xff, 0xff, 0xff, 0xff}
	colorLand := color.RGBA{0x00, 0x88, 0x22, 0xff}
	colorBorder := color.RGBA{0x22, 0x22, 0x22, 0xff}
	palette := palette.Plan9

	for lon := 360.; lon >= 0; lon -= 3 {
		projString := fmt.Sprintf("+proj=ortho +a=6378137.0 +rf=298.257223563 +towgs84=0,0,0,0,0,0,0 +lat_0=%f +lon_0=%f", 0.0, lon)
		log.Print(projString)

		projOrtho, err := proj.InitPlus(projString)
		if err != nil {
			log.Fatal(err)
		}

		transfo, err := proj.NewTransformation(projWGS84, projOrtho)
		if err != nil {
			log.Fatal(err)
		}

		img := image.NewRGBA(r)
		gc := draw2dimg.NewGraphicContext(img)

		gc.SetFillColor(colorBackground)
		gc.MoveTo(0, 0)
		gc.LineTo(imgSize, 0)
		gc.LineTo(imgSize, imgSize)
		gc.LineTo(0, imgSize)
		gc.Close()
		gc.FillStroke()

		gc.SetFillColor(colorOcean)
		gc.SetStrokeColor(colorBlack)
		gc.SetLineWidth(5)

		gc.Translate(float64(r.Dx())/2., float64(r.Dy())/2.)
		gc.Scale(imgSize/(2.*6378137.), -imgSize/(2.*6378137.))

		var globe []geom.Point
		for i := -90.; i <= 90.; i += 5. {
			globe = append(globe, geom.Point{
				X: (lon + 90) * deg2Rad,
				Y: i * deg2Rad,
			})
		}
		for i := 90.; i > -90.; i -= 5. {
			globe = append(globe, geom.Point{
				X: (lon - 90) * deg2Rad,
				Y: i * deg2Rad,
			})
		}

		if err := transfo.TransformPoints(globe); err != nil {
			log.Fatal(err)
		}
		for i := 0; i < len(globe); i++ {

			if i == 0 {
				gc.MoveTo(globe[i].X, globe[i].Y)
			} else {
				gc.LineTo(globe[i].X, globe[i].Y)
			}
		}

		gc.Close()
		gc.FillStroke()

		gc.SetFillColor(colorLand)
		gc.SetStrokeColor(colorBorder)
		gc.SetLineWidth(5)

		for i := range c.Features {

			g, err := geojson.FromGeoJSON(c.Features[i].Geometry)
			if err != nil {
				log.Fatal(err)
			}

			g.Iterate(func(points []geom.Point) error {

				for i := range points {
					points[i].X = points[i].X * deg2Rad
					points[i].Y = points[i].Y * deg2Rad
				}

				return transfo.TransformPoints(points)
			})

			g.Iterate(func(points []geom.Point) error {
				var previousX int
				var previousY int

				bStarted := false

				for i := 0; i < len(points); i++ {

					if math.IsInf(points[i].X, 0) || math.IsInf(points[i].Y, 0) {
						//Skip points that are out of the visible area
						continue
					}

					if !bStarted {
						gc.MoveTo(points[i].X, points[i].Y)

						previousX = int(points[i].X * imgSize / 6378137.)
						previousY = int(-points[i].Y * imgSize / 6378137.)

						bStarted = true

					} else {

						currentX := int(points[i].X * imgSize / 6378137.)
						currentY := int(-points[i].Y * imgSize / 6378137.)

						if currentX != previousX || currentY != previousY {
							gc.LineTo(points[i].X, points[i].Y)
							previousX = currentX
							previousY = currentY
						}

					}
				}

				if bStarted {
					gc.Close()
					gc.FillStroke()
				}

				return nil
			})

		}

		//Copy the RGBA image to a Paletted image (usable in animated GIF)
		paletted := image.NewPaletted(r, palette)
		for i := 0; i < r.Dx(); i++ {
			for j := 0; j < r.Dy(); j++ {

				paletted.Set(i, j, img.At(i, j))

			}
		}

		images = append(images, paletted)
	}

	//Save the animated GIF file
	imgFile, err := os.Create("earth.gif")
	if err != nil {
		log.Fatal(err)
	}

	delays := make([]int, len(images))
	for i := range delays {
		delays[i] = 1
	}

	g := gif.GIF{
		Image: images,
		Delay: delays,
	}

	err = gif.EncodeAll(imgFile, &g)
	if err != nil {
		log.Fatal(err)
	}
}
