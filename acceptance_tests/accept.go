package main

import (
	proc "github.com/nci/gsky/processor"
	"bufio"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var wms_caps string = "http://%s/ows?service=WMS&version=1.3.0&request=GetCapabilities"
var wps_caps string = "http://%s/ows?service=WPS&request=GetCapabilities&version=1.0.0"
var wps_descr string = "http://%s/ows?service=WPS&request=DescribeProcess&version=1.0.0&Identifier=geometryDrill"
var passed string = "Passed"
var failed string = "Failed"

func Capabilities(host, req string) bool {
	resp, err := http.Get(fmt.Sprintf(req, host))
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 200 {
		return false
	}

	return true
}

func WMS(host, urlList string, concLevel int) (bool, time.Duration) {
	out := true
	start := time.Now()
	f, err := os.Open(urlList)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	//Concurrency set to 6 simultaneous connections
	conc := proc.NewConcLimiter(concLevel)
	results := make(chan int)
	defer close(results)
	go func() {
		for res := range results {
			if res != 200 {
				out = false
			}
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		conc.Increase()
		go func(url string) {
			resp, err := http.Get(fmt.Sprintf(url, host))
			if err != nil {
				log.Fatal(err)
			}
			results <- resp.StatusCode
			conc.Decrease()
		}(scanner.Text())
	}

	conc.Wait()

	return out, time.Since(start)
}

func WPS(host, payloadPath string, concLevel int) (bool, time.Duration) {
	start := time.Now()

	out := true

	conc := proc.NewConcLimiter(concLevel)
	results := make(chan bool)
	defer close(results)
	go func() {
		for res := range results {
			if res == false {
				out = false
			}
		}
	}()

	files, _ := ioutil.ReadDir(payloadPath)
	for _, fName := range files {
		conc.Increase()
		go func(fPath string) {
			results <- QueryPolygon(host, fPath)
			conc.Decrease()
		}(payloadPath + fName.Name())
	}
	conc.Wait()
	time.Sleep(100 * time.Millisecond)

	return out, time.Since(start)
}

func QueryPolygon(host, fileName string) bool {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	resp, err := http.Post(fmt.Sprintf("http://%s/ows?service=WPS&request=Execute", host), "text/plain;charset=UTF-8", f)
	if resp.StatusCode != 200 {
		return false
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if len(body) < 10000 {
		fmt.Println(string(body))
		return false
	}

	return true
}

func inRed(str string) string {
	return fmt.Sprintf("\x1b[31;1m%s\x1b[0m", str)
}

func inGreen(str string) string {
	return fmt.Sprintf("\x1b[32;1m%s\x1b[0m", str)
}

func main() {
	host := flag.String("h", "gsky.nci.org.au", "OWS host name or address")
	suite := flag.String("s", "wms", "Test suite [wps, wms, usgs]")
	conc := flag.Int("n", 6, "Concurrency level for acceptance tests")
	flag.Parse()

	var t time.Duration
	var ok bool

	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		passed = inGreen(passed)
		failed = inRed(failed)
	}

	switch *suite {
	case "wms":
		fmt.Printf("Testing WMS GetCapabilities: ")
		if !Capabilities(*host, wms_caps) {
			fmt.Println(failed)
			os.Exit(1)
		}
		fmt.Println(passed)

		fmt.Printf("Testing WMS GetMap Sending 500 requests: ")
		if ok, t = WMS(*host, "acpt_url.tpl", *conc); !ok {
			fmt.Println(failed)
			os.Exit(1)
		}
		fmt.Println(passed, t)
	case "usgs":
		fmt.Printf("Testing WMS GetCapabilities: ")
		if !Capabilities(*host, wms_caps) {
			fmt.Println(failed)
			os.Exit(1)
		}
		fmt.Println(passed)

		fmt.Printf("Testing WMS GetMap Sending 635 requests: ")
		if ok, t = WMS(*host, "acpt_url_usgs.tpl", *conc); !ok {
			fmt.Println(failed)
			os.Exit(1)
		}
		fmt.Println(passed, t)
	case "wps":
		fmt.Printf("Testing WPS GetCapabilities: ")
		if !Capabilities(*host, wps_caps) {
			fmt.Println("\x1b[31;1mFailed\x1b[0m")
			os.Exit(1)
		}
		fmt.Println(passed)

		fmt.Printf("Testing WPS DescrProcess: ")
		if !Capabilities(*host, wps_descr) {
			fmt.Println(failed)
			os.Exit(1)
		}
		fmt.Println(passed)

		fmt.Printf("Testing WPS Polygon Drill: ")
		if ok, t = WPS(*host, "polygon_requests/", *conc); !ok {
			fmt.Println(failed)
			os.Exit(1)
		}
		fmt.Println(passed, t)
	}
}
