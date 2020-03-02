// Grab lines of scroll window. page down and repeat until page does not change.
// Then grab last line, single line down, grab new line if different or stop.

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"unsafe"
)

const globalNofBitmaps int = 200 // start with more than we will need

type bitmapSourceInfo struct {
	Character      string
	Width          int
	Height         int
	XOffset        int
	YOffset        int
	SourceFileName string
	FontFileName   string
}

type bitmapSave struct {
	Character uint8
	Width     int
	Height    int
	Pixels    []uint32
}

// saveLinesToPNG() is used to debug any problems with a grabed image
func saveLinesToPNG(imageBytes []uint32, width int, height int, fileName string) error {

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	offset := 0
	var colour color.Color
	var r uint8
	var g uint8
	var b uint8
	var pix uint32

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pix = *(*uint32)(unsafe.Pointer(&imageBytes[offset])) // this seems to work for a non 4 byte aligned memory access ... FAB
			b = (uint8)(pix & 0xFF)
			g = (uint8)((pix >> 8) & 0xFF)
			r = (uint8)((pix >> 16) & 0xFF)
			//i3 := (uint8)(pix >> 24) // this always comes back as '0'

			// Colors are defined by Red, Green, Blue, Alpha uint8 values.
			colour = color.RGBA{r, g, b, (uint8)(0xff)} // set the Alpha to 0xFF
			rgba.Set(x, y, colour)
			offset++
		}
	}

	// Encode as PNG.
	f, err := os.Create(fileName)
	if err != nil {
		log.Printf("Error creating : %v   %v", fileName, err)
		return err
		// we do not stop here, but return from the function as there may be further useful output
	} else {
		if err = png.Encode(f, rgba); err != nil {
			log.Printf("Error encoding .png, : %v", err)
			return err
		}
		f.Close()
	}

	return nil
}

func extractAndSaveFontBitmaps() error {

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	log.Println(pwd)

	// read the font bitmap info into look up array structure
	fontDescriptionData, err := ioutil.ReadFile("font_character_info.json")
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
		//	"\SourceFileName :", result["SourceFileName"])

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
		valueSourceFileName := result["SourceFileName"]
		switch v := valueSourceFileName.(type) {
		case string:
			if len(v) == 0 {
				log.Print("'SourceFileName' field empty")
				return fmt.Errorf("Font data error")
			}
			if v == "?" {
				// issue Warning / reminder ...
				log.Print("'SourceFileName' field empty needs to be filled in for # ", key)
				log.Print("'Character'        : ", n.Character)
				continue // skip saving any info for this one
			}
			n.SourceFileName = v
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'SourceFileName' field is NOT string, it's : ", result["SourceFileName"])
			log.Printf("Font index : %v", key)
			log.Printf("%v", results[key])
			return fmt.Errorf("Font data error")
		}

		// ---------------------------
		valueFontFileName := result["FontFileName"]
		switch v := valueFontFileName.(type) {
		case string:
			if len(v) == 0 {
				log.Print("'FontFileName' field empty")
				return fmt.Errorf("Font data error")
			}
			if v == "?" {
				// issue Warning / reminder ...
				log.Print("'FontFileName' field empty needs to be filled in for # ", key)
				log.Print("'Character'        : ", n.Character)
				continue // skip saving any info for this one
			}
			n.FontFileName = v
		default:
			// if the field is missing, the following will show it as <nil>
			log.Print("'FontFileName' field is NOT string, it's : ", result["FontFileName"])
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

	fontDirPath := pwd + "/font_source_bitmaps/"
	saveDirPath := pwd + "/font_bitmaps/"

	for i := 0; i < nofBitmaps; i++ {
		// copy over info for saving
		width := allFontsSource[i].Width
		height := allFontsSource[i].Height

		// check source .png file exists
		fontFile := fontDirPath + allFontsSource[i].SourceFileName
		if !fileExists(fontFile) {
			log.Print("file missing: ", fontFile)
			//			log.Print("'Character'        : ", string(uint8(allFontsSource[i].Character[0]))
			log.Print("'Character'        : ", allFontsSource[i].Character[0])
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

		extractedPixels := make([]uint32, width*height)
		var offset int
		var yPos int
		var sourcePix uint32
		var r uint8
		var g uint8
		var b uint8

		// pixels are extracted a row at a time
		for y := 0; y < height; y++ {
			yPos = (allFontsSource[i].YOffset + y) * pictureRGBA.Stride
			for x := allFontsSource[i].XOffset; x < allFontsSource[i].XOffset+width; x++ {

				sourcePix = *(*uint32)(unsafe.Pointer(&pictureRGBA.Pix[yPos+x*4]))
				// re-organise RED, GREEN, BLUE and Alpha to mathch ordering in robotgo's
				// byte array as returned from robotgo.ToBitmapBytes()
				b = (uint8)(sourcePix & 0xFF)
				g = (uint8)((sourcePix >> 8) & 0xFF)
				r = (uint8)((sourcePix >> 16) & 0xFF)

				// put bytes into order that matches raw data that roborgo AND xImg return
				extractedPixels[offset] = uint32(b)<<16 | uint32(g)<<8 | uint32(r) // Alpha is '0' in highest byte

				offset++
			}
		}

		saveFile := saveDirPath + allFontsSource[i].FontFileName
		if err = saveLinesToPNG(extractedPixels, width, height, saveFile); err != nil {
			return err
		}
	}

	if nofBitmaps > globalNofBitmaps {
		log.Printf("Need to increase 'globalNofBitmaps' to more than %v", nofBitmaps)
		return fmt.Errorf("Code setup error")
	}

	return nil
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

func main() {

	err := extractAndSaveFontBitmaps()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Printf("Done")
	os.Exit(0)
}
