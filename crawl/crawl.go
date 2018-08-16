package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"os"

	extr "github.com/nci/gsky/crawl/extractor"
)

func ensure(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

const DefaultConcLimit = 2

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide a path to a file or '-' for reading from stdin")
	}

	path := os.Args[1]

	concLimit := DefaultConcLimit
	approx := false

	if len(os.Args) > 2 {
		flagSet := flag.NewFlagSet("Usage", flag.ExitOnError)
		flagSet.IntVar(&concLimit, "conc", DefaultConcLimit, "Concurrent limit on processing subdatasets")
		flagSet.BoolVar(&approx, "approx", false, "Compute approximate statistics")
		flagSet.Parse(os.Args[2:])
	}

	if path == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		path = scanner.Text()
	}

	geoFile, err := extr.ExtractGDALInfo(path, concLimit, approx)
	ensure(err)

	out, err := json.Marshal(&geoFile)
	ensure(err)

	_, err = os.Stdout.Write(out)
	ensure(err)
}
