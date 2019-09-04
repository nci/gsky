package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"

	extr "github.com/nci/gsky/crawl/extractor"
	"github.com/nci/gsky/utils"
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
	approx := true
	sentinel2Yaml := false
	var configFile string
	ncMetadata := false

	if len(os.Args) > 2 {
		flagSet := flag.NewFlagSet("Usage", flag.ExitOnError)
		flagSet.IntVar(&concLimit, "conc", DefaultConcLimit, "Concurrent limit on processing subdatasets")
		var exact bool
		flagSet.BoolVar(&exact, "exact", false, "Compute exact statistics")
		flagSet.BoolVar(&sentinel2Yaml, "sentinel2_yaml", false, "Extract sentinel2 metadata from its yaml files")
		flagSet.StringVar(&configFile, "conf", "", "Crawl config file")
		flagSet.BoolVar(&ncMetadata, "nc_md", false, "Look for netCDF metadata")
		flagSet.Parse(os.Args[2:])

		approx = !exact
	}

	if path == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		path = scanner.Text()
	}

	var geoFile *extr.GeoFile
	var err error
	if sentinel2Yaml {
		geoFile, err = extr.ExtractSentinel2Yaml(path)
	} else {
		config := &extr.Config{}
		if len(configFile) > 0 {
			cfg, rErr := ioutil.ReadFile(configFile)
			ensure(rErr)
			err = utils.Unmarshal([]byte(cfg), config)
			ensure(err)
		} else {
			if ncMetadata {
				ruleSet := extr.RuleSet{
					NcMetadata:    ncMetadata,
					NameSpace:     extr.NSDataset,
					SRSText:       extr.SRSDetect,
					Proj4Text:     extr.Proj4Detect,
					Pattern:       `.+`,
					MatchFullPath: true,
					TimeAxis:      &extr.DatasetAxis{},
				}
				config.RuleSets = append(config.RuleSets, ruleSet)
			}
		}
		geoFile, err = extr.ExtractGDALInfo(path, concLimit, approx, config)
	}
	ensure(err)

	out, err := json.Marshal(&geoFile)
	ensure(err)

	_, err = os.Stdout.Write(out)
	ensure(err)
}
