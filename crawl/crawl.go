package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strconv"

	extr "github.com/nci/gsky/crawl/extractor"
)

func ensure(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

const DefaultConcLimit = 4

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Please provide a path to a file or '-' for reading from stdin")
	}

	path := os.Args[1]

	if path == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		path = scanner.Text()
	}

	concLimit := DefaultConcLimit
	if len(os.Args) > 2 {
		cLimit, err := strconv.ParseInt(os.Args[2], 10, 0)
		ensure(err)

		if cLimit <= 0 {
			cLimit = DefaultConcLimit
		}
		concLimit = int(cLimit)
	}

	geoFile, err := extr.ExtractGDALInfo(path, concLimit)
	ensure(err)

	out, err := json.Marshal(&geoFile)
	ensure(err)

	_, err = os.Stdout.Write(out)
	ensure(err)
}
