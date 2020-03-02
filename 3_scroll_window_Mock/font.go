package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/veandco/go-sdl2/sdl"
)

const globalNofFonts int = 200 // start with more than we will need

type bitmapSourceInfo struct {
	Character      string
	Width          int
	Height         int
	XOffset        int
	YOffset        int
	SourceFileName string
	FontFileName   string
}

type fontSave struct {
	Character uint8
	Width     int
	Height    int
	tex       *sdl.Texture
}

var globalFonts = make([]fontSave, globalNofFonts)

var actualNofFonts int = 0

func loadFontTexturesFromPNG(renderer *sdl.Renderer) error {

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	log.Println(pwd)

	// read the font bitmap info into look up array structure
	fontDescriptionData, err := ioutil.ReadFile("../2_create_font_PNGs/font_character_info.json")
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
		//	"\FontFileName :", result["FontFileName"])

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

	nofFonts := len(allFontsSource)

	log.Printf("Using %d Bitmaps\n", nofFonts)

	if nofFonts > globalNofFonts {
		log.Printf("Need to increase 'globalNofFonts' to more than %v", nofFonts)
		return fmt.Errorf("Code setup error")
	}

	fontBitmapsDirPath := pwd + "/../2_create_font_PNGs/font_bitmaps/"

	for i := 0; i < nofFonts; i++ {
		// copy over info for saving
		globalFonts[i].Character = uint8(allFontsSource[i].Character[0])
		globalFonts[i].Width = allFontsSource[i].Width
		globalFonts[i].Height = allFontsSource[i].Height

		// check source .png file exists
		fontFile := fontBitmapsDirPath + allFontsSource[i].FontFileName
		if !fileExists(fontFile) {
			log.Print("file missing: ", fontFile)
			log.Print("'Character'        : ", string(globalFonts[i].Character))
			return fmt.Errorf("Font data error")
		}

		// read in .png file
		globalFonts[i].tex, _, _ = textureFromPNG(renderer, fontFile)
	}

	actualNofFonts = nofFonts

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

func findCharacter(c byte) int {
	for i := 0; i < actualNofFonts; i++ {
		if globalFonts[i].Character == c {
			return i
		}
	}

	log.Printf("requested font number: %v is not present", c)
	// this is a show stopper ...
	os.Exit(88)
	return 0 // we don't get here, but compilation fails without this line
}
