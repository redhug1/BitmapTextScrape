# BitmapTextScrape

## Convert Bitmaps to Text from scroll window that does not provide cut and paste

Built with Go 1.13.4, Python 3.6+

## How this came about
Many years ago a USB temperature data Logger presented its log in an Application that did not allow select all and copy and also no export of the text within its scroll view window.

## Overview
The problem is overcome by controlling the window from where the text is screen grabbed and converted from bitmaps into text.

To demonstrate the whole process i created a sequence of programs whose purpose is self documented from their containing folder names.

## Platform
This was developed and tested on Ubuntu 18.04LTS and runs on the Desktop.

## How it all operates / Use

Clone this repository into a suitable folder. Whilst trying to get the various stages to execute, you may need to install extra files as detailed further on.

1. In folder` 1_mock_data`, run` 1_mock_data.py` to create` mock_data.csv`. This is a more general file than my original requirement that could have used this many years ago.
2. In folder` 2_create_font_PNGs`, run` 2_create_font_PNGs.go` to create font bitmaps in folder` font_bitmaps`. This utilises information in` 2_create_font_PNGs.json` to extract bitmaps from file` new_font_18.png` in folder` font_source_bitmaps` and save them as .png files in folder` font_bitmaps`.
3. In folder` 3_scroll_window_Mock`, from First terminal command line  run` 3_scroll_window_Mock.go` to present the` mock_data.csv` in a window utilising files created in the above two steps. This window responds to the keys PageUp, PageDown, Home, End and to mouse clicks within the page scroll up/down area and the single line up/down click areas. When this window has focus, press Esc to exit or move the mouse to the far left screen edge.
4. In folder` 4_extract_TEXT` from Second teminal command line run` r_extract_Text.go`. Do NOT nove the mouse whilst this runs. After some minutes you should have all of the converted text from the mock scroll window in a file called` extracted_text.csv`.
5. IN folder` 5_check_extracted_TEXT`, execute the script in a terminal as:` python 5_check_extracted_TEXT.py`
6. This stage is for testing a number of stages repeatedly to demonstrate a problem where PageDown at the very end scrolls less than a page's worth of lines and how it can be detected and what measures need to be applied to circumvent it for your use case. Read the` usage.txt` file in` 6_test_to_failure` and also the comments in the file that runs the test` 6_test_to_failure.sh` which you may need to make executable in the same folder. After this stage exits, yo may have to manually close the scroll mock window.

## Files you may need to install
Installing libraries for using ‘robotgo’ for ui automation:

	sudo apt-get install gcc libc6-dev
	sudo apt-get install libx11-dev xorg-dev libxtst-dev libpng++-dev
	sudo apt-get install xcb libxcb-xkb-dev x11-xkb-utils libx11-xcb-dev libxkbcommon-x11-dev

	sudo apt-get install libxkbcommon-dev
	sudo apt-get install xsel xclip

Then, the go library:

	go get github.com/go-vgo/robotgo

go: SDL graphics lib

	go get github.com/veandco/go-sdl2/sdl

    sudo apt-get install libsdl-image1.2-dev

    sudo apt install apt-file

    sudo apt install libsdl2-image-dev

go: X Go Binding

    go get github.com/robotn/xgb

## Making adjustments
There are abundant comments in the source files to assist modification to your specific requirements.
Some specific points:

1. In` 4_extract_Text.go`, some of the code has been hard wired for speed for the example font.
2. See the [Technical Notes](/docs/technical-notes.txt).
3. See [Screen Shot](/docs/Running_scroll_window_Mock.png) of the scroll window Mock as a starting point for crafting your own scroll Mock to assist in adjusting` 4_extract_Text.go` to extract text from your specific application. Its best to to create the mock and test it to match what you are wishing to grab first so that you have a HIGH Degree of Confidence that the grabing of your desired text is accurate ...

## Applications of use in making adjustments
* showing mouse co-ordinates:
` sudo apt install xdotool`
then in terminal:
` while true; do clear; xdotool getmouselocation; sleep 0.1; done`
* ` Magnus` - to see more closely where the mouse is over the mock scroll window
* ` Pinta` - great for zooming into file` new_font_18.png` to figure out the exact co-ordinates for font bitmap extraction
* ` Meld` - good for comparing` last_lines.csv` against` extracted_text.csv` if there is a problem reported.

## Performance
The bulk of the time is spent instructing ths scroll window to PageDown & then checking that its finished doing its window update. The conversion of the Bitmap into Text is extremely fast ...

For different sizes of mock data, timings on the development machine are:

* 42,761 lines (852 pages of 50 lines, then single scrolls) in 4 minutes and 54 seconds.
* 213,805 lines (4257 pages of 50 lines, then single scrolls) in 26 minutes and 47 seconds

The timings do not scal linearly because as the number of lines grows, the last mouse click actioned PageDown ends up with more and more single line scroll's left to do which from testing needed a bit longer to determine that the scroll had finished (don't know why, thats just the way i got it to work reliably).

### Licence

Copyright ©‎ 2020, red

Released under MIT license, see [LICENSE](LICENSE.md) for details.