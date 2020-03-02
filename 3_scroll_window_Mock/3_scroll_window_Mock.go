package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	screenWidth  = 550
	screenHeight = 964
)

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func textureFromBMP(renderer *sdl.Renderer, filename string) *sdl.Texture {
	img, err := sdl.LoadBMP(filename)
	if err != nil {
		panic(fmt.Errorf("loading %v: %v", filename, err))
	}
	defer img.Free()
	tex, err := renderer.CreateTextureFromSurface(img)
	if err != nil {
		panic(fmt.Errorf("creating texture from %v: %v", filename, err))
	}

	return tex
}

func textureFromPNG(renderer *sdl.Renderer, filename string) (*sdl.Texture, int32, int32) {
	img, err := img.Load(filename)
	if err != nil {
		panic(fmt.Errorf("Problem loading .PNG %v: %v", filename, err))
	}
	defer img.Free()
	tex, err := renderer.CreateTextureFromSurface(img)
	if err != nil {
		panic(fmt.Errorf("Failed creating texture from %v: %v", filename, err))
	}

	// This is for getting the Width and Height of img. Once img.Free() is called we lose the
	// ability to get information about the image we loaded into ram
	imageWidth := img.W
	imageHeight := img.H

	return tex, imageWidth, imageHeight
}

func saveBoxToPNG(width int, height int, fileName string) error {

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	var colour color.Color

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Colors are defined by Red, Green, Blue, Alpha uint8 values.
			colour = color.RGBA{0xff, 0xff, 0, (uint8)(0xff)} // set the Alpha to 0xFF
			rgba.Set(x, y, colour)
		}
		colour = color.RGBA{0, 0, 0x7f, (uint8)(0xff)} // set the Alpha to 0xFF
		rgba.Set(0, y, colour)
		rgba.Set(width-1, y, colour)
	}
	// Colors are defined by Red, Green, Blue, Alpha uint8 values.
	colour = color.RGBA{0, 0, 0x7f, (uint8)(0xff)} // set the Alpha to 0xFF
	for x := 0; x < width; x++ {
		rgba.Set(x, 0, colour)
		rgba.Set(x, height-1, colour)
	}

	// Encode as PNG.
	f, err := os.Create(fileName)
	if err != nil {
		log.Printf("Error creating : %v   %v", fileName, err)
		return err
		// we do not stop here, but return from the function as there may be further useful output
	}
	defer f.Close()

	if err = png.Encode(f, rgba); err != nil {
		log.Printf("Error encoding .png, : %v", err)
		return err
	}

	return nil
}

func textureBox(renderer *sdl.Renderer, width int, height int) (*sdl.Texture, int32, int32) {
	boxName := "scroll_box.png"
	if err := saveBoxToPNG(width, height, boxName); err != nil {
		log.Printf("%v", err)
		os.Exit(99)
	}
	// leave the file we just created in the file system, should if need to be inspected
	return textureFromPNG(renderer, boxName)
}

var allLinesReverse []string // put on global heap

const lineOffsetX = 1
const lineOffsetY = 63
const lineDepth = 18

var lineWhiteImg *sdl.Texture
var lineWhiteWidth, lineWhiteHeight int32

func renderLineBackground(renderer *sdl.Renderer, down int32) {
	renderer.Copy(lineWhiteImg,
		&sdl.Rect{X: 0, Y: 0, W: lineWhiteWidth, H: lineWhiteHeight},
		&sdl.Rect{X: int32(lineOffsetX), Y: int32(lineOffsetY + down*lineDepth), W: lineWhiteWidth, H: lineWhiteHeight})
}

func calcLineTextWidth(lineText string) int {
	var width int

	for i := 0; i < len(lineText); i++ {
		c := lineText[i]
		fontIndex := findCharacter(c)
		width += globalFonts[fontIndex].Width
	}

	return width
}

func renderLineText(renderer *sdl.Renderer, lineText string, down int32, oddLine bool) {
	parts := strings.Split(lineText, ",")
	if len(parts) != 5 {
		log.Printf("line does not have 5 sections, line %v, %v", down, lineText)
		os.Exit(110)
	}

	var xOffset = 1
	// do the Time
	for i := 0; i < len(parts[0]); i++ {
		c := parts[0][i]
		fontIndex := findCharacter(c)

		renderer.Copy(globalFonts[fontIndex].tex,
			&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)},
			&sdl.Rect{X: int32(xOffset + lineOffsetX), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)})

		xOffset += globalFonts[fontIndex].Width
	}

	// add field seperator
	sepPos1 := 106
	sepPos2 := 203
	sepPos3 := 315
	sepPos4 := 442
	sep := "|"
	seperatorIndex := findCharacter(uint8(sep[0]))
	renderer.Copy(globalFonts[seperatorIndex].tex,
		&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)},
		&sdl.Rect{X: int32(sepPos1), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)})
	renderer.Copy(globalFonts[seperatorIndex].tex,
		&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)},
		&sdl.Rect{X: int32(sepPos2), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)})
	renderer.Copy(globalFonts[seperatorIndex].tex,
		&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)},
		&sdl.Rect{X: int32(sepPos3), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)})
	renderer.Copy(globalFonts[seperatorIndex].tex,
		&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)},
		&sdl.Rect{X: int32(sepPos4), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[seperatorIndex].Width), H: int32(globalFonts[seperatorIndex].Height)})

	// do the Index
	xOffset = sepPos2 - 2 - calcLineTextWidth(parts[1])
	for i := 0; i < len(parts[1]); i++ {
		c := parts[1][i]
		fontIndex := findCharacter(c)

		renderer.Copy(globalFonts[fontIndex].tex,
			&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)},
			&sdl.Rect{X: int32(xOffset + lineOffsetX), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)})

		xOffset += globalFonts[fontIndex].Width
	}

	// do the Location
	xOffset = sepPos3 - 2 - calcLineTextWidth(parts[2])
	for i := 0; i < len(parts[2]); i++ {
		c := parts[2][i]
		fontIndex := findCharacter(c)

		renderer.Copy(globalFonts[fontIndex].tex,
			&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)},
			&sdl.Rect{X: int32(xOffset + lineOffsetX), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)})

		xOffset += globalFonts[fontIndex].Width
	}

	// do the Sensor
	xOffset = sepPos4 - 2 - calcLineTextWidth(parts[3])
	for i := 0; i < len(parts[3]); i++ {
		c := parts[3][i]
		fontIndex := findCharacter(c)

		renderer.Copy(globalFonts[fontIndex].tex,
			&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)},
			&sdl.Rect{X: int32(xOffset + lineOffsetX), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)})

		xOffset += globalFonts[fontIndex].Width
	}

	// do the Value
	xOffset = 531 - calcLineTextWidth(parts[4])
	for i := 0; i < len(parts[4]); i++ {
		c := parts[4][i]
		fontIndex := findCharacter(c)

		renderer.Copy(globalFonts[fontIndex].tex,
			&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)},
			&sdl.Rect{X: int32(xOffset + lineOffsetX), Y: int32(lineOffsetY + down*lineDepth), W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)})

		xOffset += globalFonts[fontIndex].Width
	}
}

var doRedraw = true
var lineOffset = 0

const linesOnPage = 50

var nofMockLines int // number of lines in file to be displayed

const scrollAreaSizeY = 870 // scroll bar number of pixels high
var scrollBarRatio float32
var scrollGripSize float32

const scrollGripWidth int32 = 14
const scrollGripSizeMin float32 = 8.0

var scrollPosMinY int32 = 78
var scrollPosOffset int32 = 0
var scrollPosY int32 = scrollPosMinY + scrollPosOffset

func lineDown() {
	if lineOffset < (nofMockLines - linesOnPage) {
		lineOffset++
	}
}

func lineUp() {
	if lineOffset > 0 {
		lineOffset--
	}
}

func pageDown() {
	lineOffset += linesOnPage
	if lineOffset > (nofMockLines - linesOnPage) {
		lineOffset = nofMockLines - linesOnPage
	}
}

func pageUp() {
	lineOffset -= linesOnPage
	if lineOffset < 0 {
		lineOffset = 0
	}
}

// scroll bar maths from excellent article:  http://csdgn.org/article/scrollbar

func initScrollBar() {
	scrollBarRatio = float32(linesOnPage) / float32(nofMockLines)

	scrollGripSize = scrollAreaSizeY * scrollBarRatio

	if scrollGripSize < scrollGripSizeMin {
		scrollGripSize = scrollGripSizeMin
	}

	if scrollGripSize > scrollAreaSizeY {
		scrollGripSize = scrollAreaSizeY
	}
}

func calcScrollBarPos() {
	// range that can be scrolled over ...
	actualSize := float32(nofMockLines - linesOnPage)
	if actualSize < 1.0 {
		actualSize = 1.0 // NOTE: this is possibly wrong and needs figuring out
	}
	// positional ratio ...
	positionRatio := float32(lineOffset) / actualSize

	// keep scroll within its area
	scrollSize := scrollAreaSizeY - scrollGripSize

	scrollPosOffset = int32(scrollSize * positionRatio)

	scrollPosY = scrollPosMinY + scrollPosOffset
}

func main() {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		fmt.Println("initializing SDL:", err)
		return
	}

	window, err := sdl.CreateWindow(
		"scroll window Mock",
		//		sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		10, 14,
		screenWidth, screenHeight,
		//		sdl.WINDOW_OPENGL)
		sdl.WINDOW_BORDERLESS)
	if err != nil {
		fmt.Println("initializing window:", err)
		return
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		fmt.Println("initializing renderer:", err)
		return
	}
	defer renderer.Destroy()

	var mockFile string
	flag.StringVar(&mockFile, "mock", "../1_mock_data/mock_data.csv", "path and file name to mock data file")

	flag.Parse()

	lines, err := readLines(mockFile)
	if err != nil {
		log.Fatalf("readLines: %s", err)
		os.Exit(5)
	}
	nofMockLines = len(lines)
	log.Printf("nofMockLines : %v", nofMockLines)

	initScrollBar()
	calcScrollBarPos()

	textureScroll, _, _ := textureBox(renderer, int(scrollGripWidth), int(scrollGripSize))

	// put in reverse chronological order (compared to the order of data in file),
	// so as to look back in time ... adjust to suit your application domain
	for i := nofMockLines - 1; i >= 0; i-- {
		allLinesReverse = append(allLinesReverse, lines[i])
	}

	img.Init(img.INIT_PNG)

	textureBackgroundImg, _, _ := textureFromPNG(renderer, "sprites/w8.png")

	if err = loadFontTexturesFromPNG(renderer); err != nil {
		log.Printf("font problem: %s", err)
		os.Exit(5)
	}

	lineWhiteImg, lineWhiteWidth, lineWhiteHeight = textureFromPNG(renderer, "sprites/lineWhite18.png")

	isRunning := true

	downPressed := false
	upPressed := false
	pageDownPressed := false
	pageUpPressed := false
	pageEndPressed := false
	pageHomePressed := false

	var mouseX int32
	var mouseY int32
	mouseMoved := false // mouse position only displayed after mouse has entered mock window
	mousePosUpdateLimiter := time.Tick(50 * time.Millisecond)

	for isRunning {
		currentLineOffset := lineOffset

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				isRunning = false
			case *sdl.KeyboardEvent:
				switch t.Keysym.Sym {
				case sdl.K_ESCAPE:
					isRunning = false
				case sdl.K_DOWN:
					if downPressed == false {
						// process key pressed
						downPressed = true
						lineDown()
					} else {
						// do nothing for key released
						downPressed = false
					}
				case sdl.K_UP:
					if upPressed == false {
						upPressed = true
						lineUp()
					} else {
						upPressed = false
					}
				case sdl.K_PAGEDOWN:
					if pageDownPressed == false {
						pageDownPressed = true
						pageDown()
					} else {
						pageDownPressed = false
					}
				case sdl.K_PAGEUP:
					if pageUpPressed == false {
						pageUpPressed = true
						pageUp()
					} else {
						pageUpPressed = false
					}
				case sdl.K_END:
					if pageEndPressed == false {
						pageEndPressed = true
						lineOffset = nofMockLines - linesOnPage
					} else {
						pageEndPressed = false
					}
				case sdl.K_HOME:
					if pageHomePressed == false {
						pageHomePressed = true
						lineOffset = 0
					} else {
						pageHomePressed = false
					}
				}
			case *sdl.MouseMotionEvent: // for mouse movements within the mock window
				//fmt.Printf("[%d ms] MouseMotion\ttype:%d\tid:%d\tx:%d\ty:%d\txrel:%d\tyrel:%d\n",
				//	t.Timestamp, t.Type, t.Which, t.X, t.Y, t.XRel, t.YRel)
				mouseX = t.X
				mouseY = t.Y
				doRedraw = true
				mouseMoved = true
			case *sdl.MouseButtonEvent:
				//fmt.Printf("[%d ms] MouseButton\ttype:%d\tid:%d\tx:%d\ty:%d\tbutton:%d\tstate:%d\n",
				//	t.Timestamp, t.Type, t.Which, t.X, t.Y, t.Button, t.State)
				if t.State == 1 { // button pressed ...
					// (could check and latch that it was first pressed in the correct area,
					//  and clearing any latches when not clicked in a latches area)
					if (t.X >= 534) && (t.X <= 547) {
						if (t.Y >= 64) && (t.Y <= 77) {
							lineUp()
						} else if (t.Y >= 948) && (t.Y <= 961) {
							lineDown()
						} else if (t.Y >= 79) && (t.Y <= 946) {
							if t.Y < scrollPosY {
								pageUp()
							} else if t.Y > (scrollPosY + int32(scrollGripSize)) {
								pageDown()
							}
						}
					}
				}
				//			case *sdl.MouseWheelEvent:
				//				fmt.Printf("[%d ms] MouseWheel\ttype:%d\tid:%d\tx:%d\ty:%d\n",
				//					t.Timestamp, t.Type, t.Which, t.X, t.Y)
			}
		}
		if currentLineOffset != lineOffset {
			doRedraw = true
		}

		//renderer.SetDrawColor(255, 255, 255, 255)
		//renderer.Clear()

		if doRedraw {
			doRedraw = false

			// background image ...
			renderer.Copy(textureBackgroundImg,
				&sdl.Rect{X: 0, Y: 0, W: screenWidth, H: screenHeight},
				&sdl.Rect{X: int32(0), Y: int32(0), W: screenWidth, H: screenHeight})

			// show mouse position
			// (Essential for determining the bounding box constants in "case *sdl.MouseButtonEvent" above.
			//  Also using Magnus (on Ubuntu) at x5 magnification helps a great deal)
			select {
			case <-mousePosUpdateLimiter:
				// update no faster than once every 0.05 seconds
				if mouseMoved {
					mousePos := fmt.Sprintf("%v,%v", mouseX, mouseY)
					dispX := 100 - calcLineTextWidth(mousePos)
					for i := 0; i < len(mousePos); i++ {
						c := mousePos[i]
						fontIndex := findCharacter(c)

						renderer.Copy(globalFonts[fontIndex].tex,
							&sdl.Rect{X: 0, Y: 0, W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)},
							&sdl.Rect{X: int32(dispX), Y: int32(2), W: int32(globalFonts[fontIndex].Width), H: int32(globalFonts[fontIndex].Height)})

						dispX += globalFonts[fontIndex].Width
					}
				}
			}

			for screenLine := 0; screenLine < linesOnPage; screenLine++ {
				theLine := lineOffset + screenLine
				renderLineBackground(renderer, int32(screenLine))
				renderLineText(renderer, allLinesReverse[theLine], int32(screenLine), false)
			}

			// scroll bar ...
			calcScrollBarPos()
			renderer.Copy(textureScroll,
				&sdl.Rect{X: 0, Y: 0, W: scrollGripWidth, H: int32(scrollGripSize)},
				&sdl.Rect{X: int32(555 - 21), Y: scrollPosY, W: scrollGripWidth, H: int32(scrollGripSize)})

			renderer.Present()
		} else {
			// prevent 100% CPU thread spinning if no SDL events to process
			time.Sleep(10 * time.Millisecond)
		}
	}

	// free the texture memory

	lineWhiteImg.Destroy()

	textureBackgroundImg.Destroy()
	textureScroll.Destroy()

	// ... could also destroy all the font texture's ...

	// shut sdl stuff down
	img.Quit()
	renderer.Destroy()
	window.Destroy()

	sdl.Quit()
}
