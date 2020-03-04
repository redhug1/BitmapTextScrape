// Grab lines of scroll window. page down and repeat until page does not change.
// Then grab last line, single line down, grab new line if different or stop.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-vgo/robotgo"
	"github.com/robotn/xgb"
	"github.com/robotn/xgb/xproto"
)

var (
	gatherCharacterCountsDefault int = 1 // only use this once in a while to check if search order is 'optimal'
	priorKnowledgeSpeedupDefault int = 1 // only use this if first 'x' columns of fonts are unique
	checkLastButOnePage          int = 1 // do additional check, to catch PageDown problem
	pageDownOffsetDefault        int = 9 // relative position of mouse clicks to achieve a PageDown

	// 'pageDownOffset' set to 9 is optimal for example 'mock_data.csv' of 42761 lines
	// But ... if the number of lines being grabbed falls below ~ 10600 then 'pageDownOffset' will need increasing.
)

type extractConfig struct {
	GatherCharacterCounts int `json:"GatherCharacterCounts"` // 0 or 1
	PriorKnowledgeSpeedup int `json:"PriorKnowledgeSpeedup"` // 0 or 1
	CheckLastButOnePage   int `json:"CheckLastButOnePage"`   // 0 or 1
	PageDownOffset        int `json:"PageDownOffset"`
}

var (
	mutex      sync.Mutex
	charCounts = [256]uint64{}
)

// This is prior knowledge of what we are searching for and is 'domain' specific
var extractionList = []byte{'^', '|', '0', '1', '4', '5', '3', '.', ':',
	'2', '8', '9', '7', '6', ',', '%', '+', '-'}

const mockWindowSearchPNG string = "scroll_mock.png"

const globalNofBitmaps int = 200 // start with more than we will need

type conversionResult struct {
	index int
	text  string
}

type bitmapSourceInfo struct {
	Character string
	Width     int
	Height    int
	XOffset   int
	YOffset   int
	FileName  string
}

type bitmapSave struct {
	Character uint8
	Width     int
	Height    int
	Pixels    []uint32
}

const (
	conversionGood = iota + 1 // start at 1
	conversionErrorOnlyFourDividers
	conversionErrorUnknownPixel
	conversionErrorWrongNumberOfSections
	conversionErrorTimeFormatWrong
	conversionErrorBlankLine
)

var allLines []string // put on global heap

var allLinesReverse []string // put on global heap

func getConfig(filename string) (extractConfig, error) {
	conf := extractConfig{
		gatherCharacterCountsDefault,
		priorKnowledgeSpeedupDefault,
		checkLastButOnePage,
		pageDownOffsetDefault,
	}
	file, err := os.Open(filename)
	if err != nil {
		log.Println("Configuration file not found. Continuing with default values.")
		return conf, err
	}
	err = json.NewDecoder(file).Decode(&conf)
	file.Close()
	if conf.PageDownOffset < 9 {
		conf.PageDownOffset = 9 // any smaller than 9 and PageDown does not happen for the mouse clicks for PageDown
	}
	if conf.CheckLastButOnePage != 1 {
		log.Println("WARNING: The last Page Down may scroll less than a page worth of lines and the")
		log.Println("         check to find this problem is disabled !")
	}
	return conf, err
}

// saveLinesToPNG() is used to debug any problems with a grabed image
func saveLinesToPNG(imageBytes []byte, startLineNumber int, endLineNumber int, width int, height int, fileName string) {
	// lines to save are inclusive of startLineNUmber to endLineNumber
	var linesToSave int = endLineNumber - startLineNumber + 1

	rgba := image.NewRGBA(image.Rect(0, 0, width, height*linesToSave))

	// this version of saveLinesToPNG works with an xImg.date[] as the source

	offset := startLineNumber * height * width * 4
	var colour color.Color
	var r uint8
	var g uint8
	var b uint8
	var pix uint32

	for line := 0; line < linesToSave; line++ {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				pix = *(*uint32)(unsafe.Pointer(&imageBytes[offset])) // this seems to work for a non 4 byte aligned memory access ... FAB
				b = (uint8)(pix & 0xFF)
				g = (uint8)((pix >> 8) & 0xFF)
				r = (uint8)((pix >> 16) & 0xFF)
				//i3 := (uint8)(pix >> 24) // this always comes back as '0'

				// Colors are defined by Red, Green, Blue, Alpha uint8 values.
				colour = color.RGBA{r, g, b, (uint8)(0xff)} // set the Alpha to 0xFF
				rgba.Set(x, y+(height*line), colour)
				offset += 4
			}
		}
	}

	// Encode as PNG.
	f, err := os.Create(fileName)
	if err != nil {
		log.Printf("Error creating : %v   %v", fileName, err)
		// we do not stop here, but return from the function as there may be further useful output
	} else {
		if err = png.Encode(f, rgba); err != nil {
			log.Printf("Error encoding .png, : %v", err)
		}
		f.Close()
	}
}

func bitmapToString(imageBytes []byte, lineNumber int, lineWidth int, height int, priorKnowledgeSpeedup int, gatherCharacterCounts int) conversionResult {

	// the bytes are extracted directly from imageBytes with no offset as the data from the xImg.Data[]
	// is a pixel data only array.

	// As this is a self contained App, and for MAX speed it is assumed that all input parameters to this
	// function are within bounds.

	// imageBytes[] is only read from, so its use has no concurrency issues when this function
	// is called from multiple go routines.

	var yDownStart int = 2 // all font bitmaps to be searched for start 2 pixels down (a hardwired optimisation)

	var maxFontHeight int = 13 // this is defined to clip the last line of a comma (a hardwired optimisation)
	var lineAsUint32 = make([]uint32, lineWidth*maxFontHeight)

	// Generate an array of the pixels for quick comparison
	// We copy each column of pixels as a uint32 into one long array, consecutively
	var offset int
	var yPos int

	var nofColumnsExtracted int

	var stride int = lineWidth * 4 // 4 bytes per pixel
	baseOffset := 0
	baseOffset += lineNumber * height * lineWidth * 4 // add line offset

	// Pixels are extracted a column at a time.
	//
	offset = 0
	for x := 0; x < lineWidth; x++ {
		nofColumnsExtracted++
		yPos = baseOffset + (yDownStart * stride) + (x * 4) // initialise row for start of each column
		for y := 0; y < maxFontHeight; y++ {
			lineAsUint32[offset] = *(*uint32)(unsafe.Pointer(&imageBytes[yPos]))
			yPos += stride // advance to the next row
			offset++
		}
	}

	// ====================

	// search for bitmap match
	var columnOffsetIntoLine int = 0
	var found bool

	var lineText string = ""

	var res conversionResult
	res.index = lineNumber

	//var fontLength int
	var lineOffset int

	//var bitmapSame bool
	var fontOffset int

	var xCount int

	for columnOffsetIntoLine < nofColumnsExtracted {
		found = false
		for b := 0; b < actualNofBitmaps; b++ {
			//bitmapSame = true
			if (globalBitmaps[b].Width + columnOffsetIntoLine) <= nofColumnsExtracted {
				var w = globalBitmaps[b].Width
				if priorKnowledgeSpeedup == 1 {
					if w > 4 {
						// (this has been seen to save ~ 1/6th of search time)
						w = 4 // minimum number of columns that work for fonts used
					}
				}
				//fontLength = w * globalBitmaps[b].Height
				lineOffset = columnOffsetIntoLine * maxFontHeight
				/*			for fontOffset := 0; fontOffset < fontLength; fontOffset++ {
							if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
								bitmapSame = false
								break
							}
							lineOffset++
						}*/
				fontOffset = 0
				for w1 := 0; w1 < w; w1++ {
					// For maximum speed ...
					// Unroll the inner loop 13 times (the height of font i'm searching for)
					// This will fail to do the job properly (or crash) if font height is NOT 13 !
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
					if globalBitmaps[b].Pixels[fontOffset] != lineAsUint32[lineOffset] {
						goto notSame
					}
					fontOffset++
					lineOffset++
				}
				// if bitmapSame {
				columnOffsetIntoLine += globalBitmaps[b].Width
				if globalBitmaps[b].Character != '^' {
					lineText += string(globalBitmaps[b].Character)
				} else if gatherCharacterCounts == 1 {
					xCount++ // accumulate for assignment to global variable outside of inner loop
					// NOTE: if the above was incrementing the global count for '^' under 'mutex' protection
					//       bitmapToString() runs ~3.5 times slower in dubugger
				}
				found = true
				break
				//}
			notSame:
			}
		}
		if found == false {
			// somehow the first column on a line gets messed up ... so try the next column
			// possibly the images are not aligned properly ?
			columnOffsetIntoLine++
			//res.text = "error:" + strconv.Itoa(conversionErrorUnknownPixel) + ":column " + strconv.Itoa(columnOffsetIntoLine) // unknown data
			//return res
		}
	}
	if gatherCharacterCounts == 1 {
		if len(lineText) > 15 { // simple check that line is valid before processing
			mutex.Lock() // grabing and releasing mutext around following 'specific' loop results in faster execution
			for _, ch := range lineText {
				charCounts[uint8(ch)]++
			}
			charCounts[uint8('^')] += uint64(xCount)
			mutex.Unlock()
		}
	}
	if len(lineText) > 0 {
		if lineText == "||||" {
			errorDescription := "error " + strconv.Itoa(conversionErrorOnlyFourDividers) + " : Found only 4 vertical dividers - the pixel offset for the line is most likely wrong"
			log.Printf(errorDescription)
			res.text = "error:" + strconv.Itoa(conversionErrorOnlyFourDividers) + ":" // DON'T change this error number as its checked for elsewhere !
			return res
		}
		// Apply business logic to re-formulate the line into proper numerical and data format
		parts := strings.Split(lineText, "|")
		if len(parts) != 5 {
			errorDescription := fmt.Sprintf("error %v : Line should be 5 sections but it's : %v", conversionErrorWrongNumberOfSections, len(parts))
			log.Printf(errorDescription)
			log.Printf("Line is : %v", lineText)
			res.text = "error:" + strconv.Itoa(conversionErrorWrongNumberOfSections) + ":field count " + strconv.Itoa(len(parts))
			return res
		}

		// check time:
		if parts[0][2] != ':' || parts[0][5] != ':' {
			errorDescription := "error " + strconv.Itoa(conversionErrorTimeFormatWrong) + " : Time does not have 2 colon seperators"
			log.Printf(errorDescription)
			log.Printf("Line is : %v", lineText)
			res.text = "error:" + strconv.Itoa(conversionErrorTimeFormatWrong) + ":"
			return res
		}

		// Apply any transformations to any fields here ...

		res.text = parts[0] + "," + parts[1] + "," + parts[2] + "," + parts[3] + "," + parts[4]

	} else {
		errorDescription := "error " + strconv.Itoa(conversionErrorBlankLine) + " : Blank line"
		log.Printf(errorDescription)
		res.text = "error:" + strconv.Itoa(conversionErrorBlankLine) + ":"
		return res
	}

	return res
}

func totalTime(msg string) func() {
	start := time.Now()
	log.Printf("starting : %s", msg)
	return func() { log.Printf("%s took : %s", msg, time.Since(start)) }
}

var globalBitmaps = make([]bitmapSave, globalNofBitmaps)
var actualNofBitmaps int = 0

func loadFontBitmaps() error {
	defer totalTime("loadFontBitmaps")()

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	log.Println(pwd)

	// read the font bitmap info into look up array structure
	fontDescriptionData, err := ioutil.ReadFile("optimised_character_info.json")
	if err != nil {
		return err
	}

	// Declared an empty interface of type Array
	var results []map[string]interface{}

	// Unmarshal or Decode the JSON to the interface.
	err = json.Unmarshal([]byte(fontDescriptionData), &results)
	if err != nil {
		return err
	}

	log.Printf("Reading %d Bitmaps\n", len(results))

	allFontsSource := []bitmapSourceInfo{}

	for key, result := range results {
		//fmt.Println("Extracting Infomation for Bitmap :", key)
		//Reading each value by its key
		//fmt.Println("Character :", result["Character"],
		//	"\nWidth :", result["Width"],
		//	"\nHeight :", result["Height"],
		//	"\nXOffset :", result["XOffset"],
		//	"\nYOffset :", result["YOffset"],
		//	"\nFileName :", result["FileName"])

		// Validate every field, stop if problem, otherwise save
		// (skip character without filename and issue log error indicating what's missing)
		var n bitmapSourceInfo

		// ---------------------------
		valueCharacter := result["Character"]
		switch v := valueCharacter.(type) { // do "type assertion" for all required fields
		case string:
			if len(v) > 1 {
				log.Print("'Character' field can only be ONE character, NOT : ", result["Character"])
				return fmt.Errorf("Font data error")
			}
			n.Character = v
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'Character' field is NOT string, it's : ", result["Character"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			return fmt.Errorf("Font data error")
		}

		// ---------------------------
		valueFileName := result["FileName"]
		switch v := valueFileName.(type) {
		case string:
			if len(v) == 0 {
				log.Print("'FileName' field empty")
				return fmt.Errorf("Font data error")
			}
			if v == "?" {
				// issue Warning / reminder ...
				log.Print("'FileName' field empty needs to be filled in for # ", key)
				log.Print("'Character'        : ", n.Character)
				continue // skip saving any info for this one
			}
			n.FileName = v
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'FileName' field is NOT string, it's : ", result["FileName"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			return fmt.Errorf("Font data error")
		}

		// ---------------------------
		valueWidth := result["Width"]
		switch v := valueWidth.(type) {
		case float64: // the json.Unmarshal makes the int into a float64
			if v < 1.0 || v > 30.0 {
				log.Print("'Width' field can only be >= 1 AND <= 30, NOT : ", result["Width"])
				return fmt.Errorf("Font data error")
			}
			n.Width = int(v)
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'Width' field is NOT int, it's : ", result["Width"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			log.Printf("%T\n", result["Width"])
			return fmt.Errorf("Font data error")
		}

		// ---------------------------
		valueHeight := result["Height"]
		switch v := valueHeight.(type) {
		case float64: // the json.Unmarshal makes the int into a float64
			if v < 1.0 || v > 40.0 {
				log.Print("'Height' field can only be >= 1 AND <= 40, NOT : ", result["Height"])
				return fmt.Errorf("Font data error")
			}
			n.Height = int(v)
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'Height' field is NOT int, it's : ", result["Height"])
			log.Printf("%T\n", result["Height"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			return fmt.Errorf("Font data error")
		}

		// ---------------------------
		valueXOffset := result["XOffset"]
		switch v := valueXOffset.(type) {
		case float64: // the json.Unmarshal makes the int into a float64
			if v < 0.0 || v > 4000.0 {
				log.Print("'XOffset' field can only be >= 0 AND <= 4000, NOT : ", result["XOffset"])
				return fmt.Errorf("Font data error")
			}
			n.XOffset = int(v)
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'XOffset' field is NOT int, it's : ", result["XOffset"])
			log.Printf("%T\n", result["XOffset"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			return fmt.Errorf("Font data error")
		}

		// ---------------------------
		valueYOffset := result["YOffset"]
		switch v := valueYOffset.(type) {
		case float64: // the json.Unmarshal makes the int into a float64
			if v < 0.0 || v > 3000.0 {
				log.Print("'YOffset' field can only be >= 0 AND <= 3000, NOT : ", result["YOffset"])
				return fmt.Errorf("Font data error")
			}
			n.YOffset = int(v)
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'YOffset' field is NOT int, it's : ", result["YOffset"])
			log.Printf("%T\n", result["YOffset"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			return fmt.Errorf("Font data error")
		}

		allFontsSource = append(allFontsSource, n)
	}

	nofBitmaps := len(allFontsSource)

	log.Printf("Using %d Bitmaps\n", nofBitmaps)

	extractedBitmaps := make([]bitmapSave, nofBitmaps)

	fontDirPath := pwd + "/../2_create_font_PNGs/font_source_bitmaps/"

	for i := 0; i < nofBitmaps; i++ {
		// copy over info for saving
		extractedBitmaps[i].Character = uint8(allFontsSource[i].Character[0])
		extractedBitmaps[i].Width = allFontsSource[i].Width
		extractedBitmaps[i].Height = allFontsSource[i].Height
		width := extractedBitmaps[i].Width
		height := extractedBitmaps[i].Height

		// check source .png file exists
		fontFile := fontDirPath + allFontsSource[i].FileName
		if !fileExists(fontFile) {
			log.Print("file missing: ", fontFile)
			log.Print("'Character'        : ", string(extractedBitmaps[i].Character))
			return fmt.Errorf("Font data error")
		}

		// read in .png file
		infile, err := os.Open(fontFile)
		if err != nil {
			log.Print("error: Can't open font file", err)
			return fmt.Errorf("Font data error")
		}

		picture, err := png.Decode(infile)
		if err != nil {
			log.Print("error: Can't Decode .png file", err)
			infile.Close()
			return fmt.Errorf("Font data error")
		}
		infile.Close()

		pictureRGBA, _ := picture.(*image.NRGBA) // you migt need to change to '.RGBA'
		if pictureRGBA == nil {
			log.Print("file is not correct .png format")
			return fmt.Errorf("Font data error")
		}

		if pictureRGBA.Stride != picture.Bounds().Max.X*4 {
			log.Print("unsupported stride")
			return fmt.Errorf("Font data error")
		}

		alignmentBoundary := unsafe.Alignof(pictureRGBA.Pix)
		if alignmentBoundary%4 != 0 {
			log.Print("Pix data is not aligned on 4 byte boundary for *uint32 access")
			return fmt.Errorf("Font data error")
		}

		// Now we are going to extract the bytes for the pixel data as a uint32	for MAXIMUM speed
		//
		// There are a number of methods that can be used:
		// Direct look up in the array:
		//     var pixel4 [4]uint8
		//     copy(pixel4[:], pictureRGBA.Pix[:4])
		//     fmt.Printf("%#02x %#02x %#02x %#02x\n", pixel4[0], pixel4[1], pixel4[2], pixel4[3])
		// Using the Binary package:
		//     var pixel4_2nd uint32
		//     // create uint32 from 4 bytes
		//     pixel4_2nd = binary.LittleEndian.Uint32(pictureRGBA.Pix[0:])
		//     fmt.Printf("%#08x\n", pixel4_2nd)
		//
		// OR 'casting' proper ... from (end of) article : https://kokes.github.io/blog/2019/03/19/deserialising-ints-from-bytes.html
		// runs a LOT faster ...
		// (see also : https://go101.org/article/unsafe.html)
		//     var pixel_cast uint32
		//     pixel_cast = *(*uint32)(unsafe.Pointer(&pictureRGBA.Pix[0]))

		// 'casting' will be used ...

		extractedPixels := make([]uint32, width*height)
		var offset int
		var yPos int
		var sourcePix uint32
		var r uint8
		var g uint8
		var b uint8

		// pixels are extracted a column at a time
		for x := allFontsSource[i].XOffset; x < allFontsSource[i].XOffset+width; x++ {
			yPos = allFontsSource[i].YOffset*pictureRGBA.Stride + x*4
			for y := 0; y < height; y++ {

				sourcePix = *(*uint32)(unsafe.Pointer(&pictureRGBA.Pix[yPos]))
				// re-organise RED, GREEN, BLUE and Alpha to mathch ordering in robotgo's
				// byte array as returned from robotgo.ToBitmapBytes()
				b = (uint8)(sourcePix & 0xFF)
				g = (uint8)((sourcePix >> 8) & 0xFF)
				r = (uint8)((sourcePix >> 16) & 0xFF)

				// put bytes into order that matches raw data that roborgo AND xImg return
				extractedPixels[offset] = uint32(b)<<16 | uint32(g)<<8 | uint32(r) // Alpha is '0' in highest byte

				yPos += pictureRGBA.Stride
				offset++
			}
		}

		extractedBitmaps[i].Pixels = extractedPixels
	}

	// ========================================================================
	// Now re-arrange the extractedBitmaps[] into a prioritizedBitmaps[]
	// to get minimum execution time in decoding 'single lines' of bitmaps.
	// This optimisation saves maybe ~ 40% ... depends on application domain

	prioritizedBitmaps := make([]bitmapSave, nofBitmaps)

	var destination int

	for _, v := range extractionList {
		for i := 0; i < nofBitmaps; i++ {
			if extractedBitmaps[i].Character == v {
				prioritizedBitmaps[destination] = extractedBitmaps[i]
				destination++
			}
		}
	}

	// and copy the re-ordered list of structures into global array
	if nofBitmaps > globalNofBitmaps {
		log.Printf("Need to increase 'globalNofBitmaps' to more than %v", nofBitmaps)
		return fmt.Errorf("Code setup error")
	}

	for i := 0; i < nofBitmaps; i++ {
		globalBitmaps[i] = prioritizedBitmaps[i]
	}
	actualNofBitmaps = nofBitmaps

	return nil
}

func heartbeatSpinner(ctx context.Context, delay time.Duration) { // just for visual effect ... this and related code can be removed.
	var sChars = `-\|/`
	var i int
	var c int
	for {
		select {
		case <-ctx.Done():
			fmt.Printf(" \r")
			return
		case <-time.After(delay):
			c++
			if c < 160 {
				fmt.Printf("\r%c", sChars[i]) // this only works well in a terminal (not delve debug window of vscode)
			} else {
				c = 0
				fmt.Printf("\n")
			}
			i++
			if i >= len(sChars) {
				i = 0
			}
		}
	}
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// writeLines writes the lines to the given file.
func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func checkLine(textResult string) int {
	if textResult[0:5] == "error" {
		parts := strings.Split(textResult, ":")
		if len(parts) < 2 {
			log.Printf("Code error, error format must be 'error:<num>:<optional string> , NOT %v", textResult)
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(1)
		}
		num, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("Code error, the error number is not an integer: %v", parts[1])
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(2)
		}
		// Expand the error for easier reading ...
		log.Printf("Error : %v\n", num)
		if len(parts) > 2 {
			log.Printf("        %v\n", parts[2])
		}
		return num
	}

	return conversionGood
}

var mouseX, mouseY int

var ctrlC int32 = 0

func main() {
	start := time.Now()

	err := loadFontBitmaps()
	if err != nil {
		log.Println(err)
		os.Exit(3)
	}

	configPath := flag.String("config", "./configuration/config.json", "path to config file")
	flag.Parse()
	config, _ := getConfig(*configPath)

	var concurrent = runtime.NumCPU()
	if concurrent >= 8 {
		concurrent -= 2 // leave a few CPU threads free to 'scroll_mock' for optimal performance
	} else if concurrent >= 4 {
		concurrent -= 1
	}
	var semaphoreChan = make(chan struct{}, concurrent)

	signalsChan := make(chan os.Signal, 1)
	signal.Notify(signalsChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		for sig := range signalsChan {
			err = errors.New("aborting after signal")
			log.Println(err)
			log.Println(sig.String())
			atomic.StoreInt32(&ctrlC, 1) // set 'CTRL-C pressed' flag for any loops to inspect and then terminate
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// grab the mouse position, to be restorred at end (or when appropriate)
	mouseX, mouseY = robotgo.GetMousePos()

	if fileExists(mockWindowSearchPNG) == false {
		log.Println("mock window seach .PNG file missing")
		robotgo.MoveMouse(mouseX, mouseY)
		os.Exit(4)
	}

	time.Sleep(500 * time.Millisecond)

	abitMap := robotgo.CaptureScreen()

	log.Printf("searching for location image - make sure its in the top left of search window and\nthat window is in top left of screen for best speed")
	left, top := robotgo.FindPic(mockWindowSearchPNG, abitMap, 0.0) // exact match
	if (left == -1) && (top == -1) {
		robotgo.SaveCapture("saveCapture.png", 0, 0, 1000, 1000)
		log.Println("Can not find any window with searched for .PNG")
		robotgo.MoveMouse(mouseX, mouseY)
		os.Exit(5)
	} else {
		left -= 190 // for .png "scroll_mock.png"
		top -= 2
	}

	log.Println("FindBitmap...", left, top)

	log.Println("Inner Comparison loop unrolled")

	// select the list Window
	robotgo.MoveMouse(left+10, top+5)
	robotgo.Click("left", false) // 'false' for single click, 'true' for double click

	// up scroll move
	upOneRelX := 551 - left
	upOneRelY := 85 - top
	upX := left + upOneRelX
	upY := top + upOneRelY
	// ensure the window is at the top
	robotgo.MoveMouse(upX, upY)
	robotgo.Click("left", false) // 'false' for single click, 'true' for double click
	time.Sleep(250 * time.Millisecond)

	// down scroll move
	downOneRelX := 551 - left
	downOneRelY := 1113 - top
	downX := left + downOneRelX
	downY := top + downOneRelY - 146

	topLineRelX := 1
	topLineRelY := 62

	topX := left + topLineRelX
	topY := top + topLineRelY

	log.Printf("topX, topY: %v, %v\n", topX, topY)

	topWidth := 532
	log.Println("width ", topWidth)
	topHeight := 18

	const fileNamePrefix string = "lines/page_"
	pageNumber := 0
	nofGrabs := 0

	const linesShown int = 50 // exactly 50 lines and a scroll down moves exactly 50 lines
	// get and save the first image

	c, err := xgb.NewConn()
	if err != nil {
		log.Printf("xgb.NewConn FAIL")
		robotgo.MoveMouse(mouseX, mouseY)
		os.Exit(6)
	}
	defer c.Close()
	screen := xproto.Setup(c).DefaultScreen(c)

	lastxImg, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY), uint16(topWidth), uint16(topHeight*linesShown), 0xffffffff).Reply()
	if err != nil {
		log.Printf("xproto.GetImage FAIL 1")
		robotgo.MoveMouse(mouseX, mouseY)
		os.Exit(7)
	}
	nofGrabs++
	log.Println("Do NOT touch the Mouse, until this Application has finished ... (or move it to far left of screen to exit)")

	var textResult = make([]conversionResult, linesShown)
	var wg sync.WaitGroup                                           // number of working goroutines
	allConvertedTextChan := make(chan conversionResult, linesShown) // without 'linesShown' in the definition 'deadlock' happens

	// extract the data for the FIRST screen ...
	for lineNum := 0; lineNum < linesShown; lineNum++ {
		//----
		//f2 := fmt.Sprintf("%s_n_%05d.png", fileNamePrefix, lineNum)
		//saveLinesToPNG(lastxImg.Data, lineNum, lineNum, topWidth, topHeight, f2)
		//----

		semaphoreChan <- struct{}{} // block while full

		wg.Add(1)
		// Worker
		go func(lineToConvert int) {
			defer func() {
				<-semaphoreChan // read to release a slot
			}()

			defer wg.Done()

			var convertedResult conversionResult
			convertedResult = bitmapToString(lastxImg.Data, lineToConvert, topWidth, topHeight, config.PriorKnowledgeSpeedup, config.GatherCharacterCounts)
			allConvertedTextChan <- convertedResult
		}(lineNum)
	}

	// closer
	go func() {
		wg.Wait()
		close(allConvertedTextChan)
	}()

	// extract the results into correct index
	for convertedLine := range allConvertedTextChan {
		textResult[convertedLine.index] = convertedLine
	}

	var convertedResult conversionResult

	for lineNum := 0; lineNum < linesShown; lineNum++ {
		convertedResult = textResult[lineNum]
		if checkLine(convertedResult.text) == conversionGood {
			allLines = append(allLines, convertedResult.text)
		} else {
			log.Printf("Stopping, as we should not have an error in the first screen grab")
			log.Printf("Maybe the font has changed ?")
			log.Printf("Saving problem image to : error_image.png")
			saveLinesToPNG(lastxImg.Data, lineNum, lineNum, topWidth, topHeight, "error_image.png")
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(8)
		}
	}

	pageNumber++

	sameCount := 0
	totalPartialCount := 0

	var delayForPages = 0
	// scroll window down one page
	robotgo.MoveMouse(downX, downY-config.PageDownOffset)
	robotgo.Click("left", false)       // 'false' for single click, 'true' for double click
	time.Sleep(100 * time.Millisecond) // give mouse click action time to get update done
	delayForPages += 100

	ctx, cancelHeartbeat := context.WithCancel(context.Background())

	go heartbeatSpinner(ctx, 75*time.Millisecond)

	var imageSame int

	// NOTE: The timing delays in the following seem to give the correct results.
	//       Any quicker and the results are wrong !
	//       That is 0.1 seconds after mouse click and 40ms delay for check.
	//
	//	So, leave them as they are or if needs be, make them slower.
	//

	for {
		newxImg, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY), uint16(topWidth), uint16(topHeight*linesShown), 0xffffffff).Reply()
		if err != nil {
			log.Printf("xproto.GetImage FAIL 2")
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(9)
		}
		nofGrabs++

		imageSame = bytes.Compare(lastxImg.Data, newxImg.Data)

		if imageSame == 0 { // The image is the same (took ~ 1.0x ms to do comparison for same image)
			sameCount++
			if sameCount > 10 {
				fmt.Println("\rSame image :", sameCount)
			}
			if sameCount > 25 {
				// The image has not changed for ~250ms, therefore we must be at the end of
				// 'page down' causing a scroll to happen, so mov on to next stage ...
				// It's 250ms because sometimes other background task's kick in and cause
				// significant delays
				log.Println("Do NOT touch the Mouse, until this Application has finished ...")
				break
			}
		} else {
			// (typically takes ~ < 2us for comparison to determine images are different, but sometimes ~300us
			//  ... possibly due to garbage collection)
			//
			// we potentially have a completely new image, but may have grabed it part way through
			// the other process updating its window ...
			// so we wait another 0.04 seconds, take another grab and compare again
			time.Sleep(40 * time.Millisecond)
			delayForPages += 40

			new2xImg, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY), uint16(topWidth), uint16(topHeight*linesShown), 0xffffffff).Reply()
			if err != nil {
				log.Printf("xproto.GetImage FAIL 3")
				robotgo.MoveMouse(mouseX, mouseY)
				os.Exit(10)
			}
			nofGrabs++

			imageSame = bytes.Compare(new2xImg.Data, newxImg.Data)

			if imageSame == 0 { // the second grab of image is now same
				lastxImg.Data = newxImg.Data

				var wg sync.WaitGroup                                           // number of working goroutines
				allConvertedTextChan := make(chan conversionResult, linesShown) // without 'linesShown' in the definition 'deadlock' happens

				// extract the data ...
				for lineNum := 0; lineNum < linesShown; lineNum++ {

					semaphoreChan <- struct{}{} // block while full

					wg.Add(1)
					// Worker
					go func(lineToConvert int) {
						defer func() {
							<-semaphoreChan // read to release a slot
						}()

						defer wg.Done()

						allConvertedTextChan <- bitmapToString(lastxImg.Data, lineToConvert, topWidth, topHeight, config.PriorKnowledgeSpeedup, config.GatherCharacterCounts)
					}(lineNum)
				}

				// closer
				go func() {
					wg.Wait()
					close(allConvertedTextChan)
				}()

				// insert the results into correct index
				for convertedLine := range allConvertedTextChan {
					textResult[convertedLine.index] = convertedLine
				}

				var convertedResult conversionResult

				for lineNum := 0; lineNum < linesShown; lineNum++ {
					convertedResult = textResult[lineNum]
					if checkLine(convertedResult.text) == conversionGood {
						allLines = append(allLines, convertedResult.text)
					} else {
						log.Printf("Stopping 2, as we should not have an error in page: %v", pageNumber)
						log.Printf("Maybe the font has changed ?")
						robotgo.MoveMouse(mouseX, mouseY)
						os.Exit(11)
					}
				}

				sameCount = 0

				// scroll window down one page
				robotgo.MoveMouse(downX, downY-config.PageDownOffset)
				robotgo.Click("left", false) // 'false' for single click, 'true' for double click

				pageNumber++
				time.Sleep(10 * time.Millisecond) // give mouse click action time to get update done
				delayForPages += 10
				// dynamically add additional delays depending on how many times we have added additional delays
				if totalPartialCount > 40 {
					time.Sleep(40 * time.Millisecond)
					delayForPages += 40
					totalPartialCount-- // back off delays to try and achieve optimum
				} else if totalPartialCount > 30 {
					time.Sleep(30 * time.Millisecond)
					delayForPages += 30
					totalPartialCount--
				} else if totalPartialCount > 20 {
					time.Sleep(20 * time.Millisecond)
					delayForPages += 20
					totalPartialCount--
				} else if totalPartialCount > 10 {
					time.Sleep(10 * time.Millisecond)
					delayForPages += 10
				}
			} else {
				totalPartialCount++
				// not a full update, so loop around and try again ...
			}
		}

		time.Sleep(10 * time.Millisecond) // give mouse click action time to get update done
		delayForPages += 10

		mX, _ := robotgo.GetMousePos()
		if (mX < 50) || atomic.LoadInt32(&ctrlC) == 1 {
			// the mouse has been moved to the left, OR CTRL-C detected
			log.Println("User exit")
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(12)
		}
	}

	cancelHeartbeat() // stop the heartbeatSpinner()

	fmt.Printf("\r")

	log.Printf("# of pages: %v, total delay time in ms : %v, average delay per page %.2fms", pageNumber, delayForPages, float64(delayForPages)/float64(pageNumber))
	log.Printf("nofGrabs: %v", nofGrabs)

	ctx2, cancelHeartbeat2 := context.WithCancel(context.Background())

	go heartbeatSpinner(ctx2, 75*time.Millisecond)

	// page down has gone as far as it can, so now we extract single lines ...
	sameCount = 0
	for {
		// scroll up 1 line
		robotgo.MoveMouse(downX, downY)
		if sameCount == 0 {
			robotgo.Click("left", false)
		}
		time.Sleep(250 * time.Millisecond) // give mouse click action time to get update done

		newxImg, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY), uint16(topWidth), uint16(topHeight*linesShown), 0xffffffff).Reply()
		if err != nil {
			log.Printf("xproto.GetImage FAIL 4")
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(13)
		}

		imageSame = bytes.Compare(lastxImg.Data, newxImg.Data)

		if imageSame == 0 { // The image is the same (the other application has not yet updated the page from the mouse click)
			if sameCount > 15 {
				break
			}
			sameCount++
		} else {
			time.Sleep(500 * time.Millisecond) // just to be sure
			// grab just the last line
			oneLinexImg, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY+(topHeight*(linesShown-1))), uint16(topWidth), uint16(topHeight), 0xffffffff).Reply()
			if err != nil {
				log.Printf("xproto.GetImage FAIL 6")
				robotgo.MoveMouse(mouseX, mouseY)
				os.Exit(14)
			}
			var linesSame = 0
			for m := 0; m < 5; m++ {
				time.Sleep(50 * time.Millisecond)
				oneLinexImg2, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY+(topHeight*(linesShown-1))), uint16(topWidth), uint16(topHeight), 0xffffffff).Reply()
				if err != nil {
					log.Printf("xproto.GetImage FAIL 6")
					robotgo.MoveMouse(mouseX, mouseY)
					os.Exit(15)
				}
				imageSame = bytes.Compare(oneLinexImg.Data, oneLinexImg2.Data)
				if imageSame == 0 {
					linesSame++
				}
			}
			if linesSame == 5 {
				// The image grab for the whole page is stable ...

				sameCount = 0
				lastxImg.Data = newxImg.Data

				convertedResult = bitmapToString(oneLinexImg.Data, 0, topWidth, topHeight, config.PriorKnowledgeSpeedup, config.GatherCharacterCounts)
				var checkResult int = checkLine(convertedResult.text)
				if checkResult != conversionGood {
					log.Printf("There is definately a problem with this line")
					log.Printf("Stopping 4, as we should not have an error in page: %v", pageNumber)
					log.Printf("Maybe the font has changed ?")
					log.Printf("Saving problem image to : error_image.png")
					saveLinesToPNG(oneLinexImg.Data, 0, 0, topWidth, topHeight, "error_image.png")
					robotgo.MoveMouse(mouseX, mouseY)
					os.Exit(16)
				} else if checkResult == conversionGood {
					allLines = append(allLines, convertedResult.text)
				}

				// NOTE: on one occasion a black line was grab'd
				//       BUT was not repeatable and i could not
				//       see how this could happen.
				//       ... But a black imaged saved off will later break the
				//           processing pipe line, so we STOP here and then
				//           investigate  OR  re-try ...

				var pixColour uint32
				pixOffset := ((3 * topWidth) + 100) * 4 // (3 lines down, 100 pixels across) multiplied by bytes per pixel

				pixColour = *(*uint32)(unsafe.Pointer(&oneLinexImg.Data[pixOffset])) // this seems to work for a non 4 byte aligned memory access ... FAB

				if (pixColour & 0xFFFFFF) == 0 {
					log.Printf("Pixel at 100, 3 'and' maybe line is BLACK ... it must NOT be this way\n")
					log.Printf("Re-run and if it happens again, place breakpoint here")
					log.Printf("  and runing Debugger to examine variables, etc")
					log.Printf("Saving problem image to : error_image.png")
					saveLinesToPNG(oneLinexImg.Data, 0, 0, topWidth, topHeight, "error_image.png")
					os.Exit(17)
				}

				pageNumber++

			} else {
				// The image grab for the whole page changed ...
				// but the last line has not stabilised, so go try again.
				sameCount++
			}
		}

		mX, _ := robotgo.GetMousePos()
		if (mX < 50) || atomic.LoadInt32(&ctrlC) == 1 {
			// the mouse has been moved to the left, OR CTRL-C detected
			log.Println("User exit")
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(18)
		}
	}

	cancelHeartbeat2() // stop the heartbeatSpinner()

	var lastLines []string
	var totalLastLines int

	var nofLastPagesToCheck int = 1
	if config.CheckLastButOnePage == 1 {
		nofLastPagesToCheck = 2
	}

	for nofLastPagesToCheck > 0 {
		// Grab last pages lines in reverse order to save having to reverse the list
		// These are then used as a sanity check that the last lines grabbed via single line scroll
		// have been done correctly.
		for lineNum := linesShown - 1; lineNum >= 0; lineNum-- { // starting at last line
			oneLinexImg, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(screen.Root), int16(topX), int16(topY+(topHeight*lineNum)), uint16(topWidth), uint16(topHeight), 0xffffffff).Reply()
			if err != nil {
				log.Printf("xproto.GetImage FAIL 7")
				robotgo.MoveMouse(mouseX, mouseY)
				os.Exit(19)
			}

			convertedResult = bitmapToString(oneLinexImg.Data, 0, topWidth, topHeight, config.PriorKnowledgeSpeedup, config.GatherCharacterCounts)
			if checkLine(convertedResult.text) == conversionGood {
				lastLines = append(lastLines, convertedResult.text)
			} else {
				// hmmm, not a good capture ...save for inspection to analyse problem
				log.Printf("Stopping 5, as we should not have an error in page: %v", pageNumber)
				log.Printf("Maybe the font has changed ?")
				log.Printf("Saving problem image to : error_image.png")
				saveLinesToPNG(oneLinexImg.Data, 0, 0, topWidth, topHeight, "error_image.png")
				robotgo.MoveMouse(mouseX, mouseY)
				os.Exit(20)
			}
			totalLastLines++
		}

		if config.CheckLastButOnePage == 1 && nofLastPagesToCheck > 1 {
			// scroll up a page
			robotgo.MoveMouse(upX, upY+40)
			robotgo.Click("left", false)       // 'false' for single click, 'true' for double click
			time.Sleep(500 * time.Millisecond) // should be plenty of time for the update to complete

		}
		nofLastPagesToCheck--
	}

	// save for any manual error checking
	if err := writeLines(lastLines, "last_lines.csv"); err != nil {
		log.Printf("writeLines: %s", err)
		robotgo.MoveMouse(mouseX, mouseY)
		os.Exit(21)
	}

	// put in chronological order (compared to the order of data processed) ... adjust this if not needed
	nofLines := len(allLines)
	log.Printf("nofLines : %v", nofLines)
	for i := nofLines - 1; i >= 0; i-- {
		allLinesReverse = append(allLinesReverse, allLines[i])
	}
	if err := writeLines(allLinesReverse, "extracted_text.csv"); err != nil {
		log.Printf("writeLines: %s", err)
		robotgo.MoveMouse(mouseX, mouseY)
		os.Exit(22)
	}

	// ----
	// Check last lines match
	for i := 0; i < totalLastLines; i++ {
		if lastLines[i] != allLinesReverse[i] {
			log.Printf("Line mismatch at line : %v   %s  !=  %s", i+1, lastLines[i], allLinesReverse[i])
			log.Printf("You might try increasing the value of 'PageDownOffset' by 1 in config.json and running again.")
			log.Printf("NOTE: This problem is not captured when flag 'CheckLastButOnePage' in config.json is set to '0'")
			log.Printf(" - to demonstrate, run the stage 6 script '6_test_to_failure.sh' with above flag set to '0'")
			robotgo.MoveMouse(mouseX, mouseY)
			os.Exit(23)
		}
	}

	//
	// ----	Sort and print the charCounts (effectively sorting a "map[key]value" by value)
	//      To be used to examine the distribution of characters, such that one can manually re-arrange
	//		the order of characters in extractionList[] that then speeds up the order
	//      in which characters in the globalBitmaps[] array are searched for in bitmapToString()
	if config.GatherCharacterCounts == 1 {
		type kv struct {
			Key   string
			Value uint64
		}
		var ss []kv

		for i := 0; i < 256; i++ {
			if charCounts[i] > 0 {
				ss = append(ss, kv{string(i), charCounts[i]})
			}
		}
		sort.Slice(ss, func(a, b int) bool {
			return ss[a].Value > ss[b].Value
		})
		var lineString string
		for _, kv := range ss {
			log.Printf("Character: %v  : %v", kv.Key, kv.Value)
			lineString = lineString + kv.Key + " "
		}
		log.Printf("New  %v", lineString)
		var oldString string = "Old "
		for _, c := range extractionList {
			oldString += " "
			oldString += string(c)
		}
		log.Print(oldString)
	}

	//
	// ----
	elapsed := time.Since(start)
	log.Printf("")
	log.Println(fmt.Sprintf("extracting TEXT took %s", elapsed))

	log.Printf("Capture and Conversion completed OK")

	// restore the mouse position
	robotgo.MoveMouse(mouseX, mouseY)

	os.Exit(0)
}
