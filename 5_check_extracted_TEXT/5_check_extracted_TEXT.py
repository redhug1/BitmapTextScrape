# check text grab results match original mock data

import sys
import os
import argparse

def check_files(mock_file, extracted_file):
    print("Original mock data file : ", mock_file)
    print("Extracted text data file: ", extracted_file)
    print("")

    mock = []
    mock_len = 0
    extracted = []
    extracted_len = 0

    with open(mock_file, 'r') as f:
        for line in f:
            mock.append(line)
            mock_len += 1

    with open(extracted_file, 'r') as f:
        for line in f:
            extracted.append(line)
            extracted_len += 1

    fail = 0
    if mock_len != extracted_len:
        fail = 1
        if extracted_len > mock_len:
            print("First problem: extracted data file is longer than the original mock data file")
        else:
            print("First problem: extracted data file is shorter than the original mock data file")

    compare_len = mock_len
    if extracted_len < mock_len:
        compare_len = extracted_len

    pos = 0

    while pos < compare_len:
        if mock[pos] != extracted[pos]:
            print("Extracted data differs to original mock data at line: ", pos+1)
            print("Original line  : ", mock[pos])
            print("Extractedd line: ", extracted[pos])
            sys.exit(1)

        pos += 1

    if fail == 1:
        sys.exit(2)

    print("Extracted text matches the original OK")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("-m", "--mock", help="mock data file to check against", default="../1_mock_data/mock_data.csv")
    parser.add_argument("-e", "--extracted", help="extracted text data file to check", default="../4_extract_TEXT/extracted_text.csv")

    args = parser.parse_args()

    biggest_max = 0
    biggest_line = 0

    if os.path.isfile(args.mock) and os.access(args.mock, os.R_OK):
        if os.path.isfile(args.extracted) and os.access(args.extracted, os.R_OK):
            check_files(args.mock, args.extracted)
        else:
            print("Either the 'extracted text' file is missing or not readable: ", args.extracted)
            sys.exit(3)
    else:
        print("Either the 'mock text' file is missing or not readable: ", args.mock)
        sys.exit(4)

    sys.exit(0)
