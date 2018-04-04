package main

import (
	extr "github.com/nci/gsky/crawl/extractor"
	"bufio"
	"encoding/json"
	"log"
	"os"
)

func ensure(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	if len(os.Args) != 2 {
		log.Fatal("Please provide a path to a file or '-' for reading from stdin")
	}

	path := os.Args[1]

	if path == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		path = scanner.Text()
	}

	geoFile, err := extr.ExtractGDALInfo(path)
	ensure(err)

	out, err := json.Marshal(&geoFile)
	ensure(err)

	_, err = os.Stdout.Write(out)
	ensure(err)
}
