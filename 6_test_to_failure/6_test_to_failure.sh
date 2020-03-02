#!/usr/bin/env bash
#
# This script is for testing text extraction in a loop on a reducing mock data file
# until the PageDown's scroll less than a page

# To run, this needs:
#   the original mock_data.csv from github to be in folder 1_mock_data,
#   the binary of '3_scroll_window_Mock' to exist in its 3_scroll_window_Mock folder,
#   all other files to be in their respective folders as downloaded from github,
# 	the flag 'CheckLastButOnePage' in 4_extract_TEXT/configuration/config.json is set to '0'
#
#   and ... this script is executed in a terminal from its folder 6_test_to_failure
#
# ALSO: Ensure the terminal you run this from is away from the top left of the screen !

clear

echo "Have your terminal open to show 50 or more lines to observe progress"
echo " ... and don't move the mouse !"
echo " "

set -ex # 'e' to stop on error (non zero return from script), 'x' to show command as it runs

cp ../1_mock_data/mock_data.csv mock_data_test.csv

# Reduce the number of mock'd lines to quickly demonstrate a minor problem with PageDown ...
# (Assuming using file from github that has 42761 lines in it to start with ...)
head -n -32160 mock_data_test.csv > test.csv

cp test.csv mock_data_test.csv


while true
do

    cd ../3_scroll_window_Mock

    ./3_scroll_window_Mock -mock ../6_test_to_failure/mock_data_test.csv &

    pid_scroll_mock=$!

    sleep 1     # give '3_scroll_window_Mock' time to render window (this may need to be more on slow machines)

    cd ../4_extract_TEXT

    go run 4_extract_Text.go

    kill -9 $pid_scroll_mock

    sleep 1     # a slight delay to allow the output from kill to happen before the next stage

    echo " "

    cd ../5_check_extracted_TEXT

    python3 -u 5_check_extracted_TEXT.py -m ../6_test_to_failure/mock_data_test.csv -e ../4_extract_TEXT/extracted_text.csv

    echo " "

    cd ../6_test_to_failure

    # remove last 1 lines for next loop
    head -n -1 mock_data_test.csv > test.csv

    cp test.csv mock_data_test.csv

done
