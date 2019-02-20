package processor
// AVS: Functions to connect GSKY to Thredds
/*
	- This program must reside in gsky/processor.
		- No other package is required.
		- Calls to the functions in this package are only in 'processor/tile_indexer.go'
	- Alter these lines to denote the Thredds dir and URL:
		var	baseThreddsDataDir = "/usr/local/tds/apache-tomcat-8.5.35/content/thredds/public/gsky/"
		var baseThreddsUrl = "http://localhost:8080/thredds/catalog/gsky/"
	- Each user will create a subdir, held in a permanent cookie, to hold their NC files.
		- Cookies can only be set if the domain is real. 'localhost' and '127.0.0.1' cannot set cookies.
		- Hence, a subdir called 'localhost' will be used for these.
	- The Thredds URL will be constructed from the user's specific subir name
*/
import (
	"fmt"
	"io/ioutil"
	"time"
	"strconv"
	"os/exec"
	"os"
	"regexp"
	"net/http"
	"math/rand"
    "strings"
)
var	baseThreddsDataDir = "/usr/local/tds/apache-tomcat-8.5.35/content/thredds/public/gsky/"
var baseThreddsUrl = "http://localhost:8080/thredds/catalog/gsky/"
var ThreddsUrl = baseThreddsUrl
var ThreddsDataDir = baseThreddsDataDir
var AddToThredds = 0 // Required to prevent anything other than 'request=GetMap'to call thredds
var nc_added = 0
var Referer = ""
func setNewTimestamp (subdir string, w http.ResponseWriter){
		expiry_date := time.Date(2050, 12, 31, 23, 59, 59, 0, time.Local) // Saturday, December 31, 2050 at 11:59:59 PM
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
    	cookie_value := subdir + ":" + timestamp
		cookie := &http.Cookie{
			Name:  "Thredds_session",
			Value: cookie_value,
			Expires: expiry_date,
		}
//fmt.Println(cookie)
		http.SetCookie(w, cookie)
		ThreddsDataDir = baseThreddsDataDir + subdir
		ThreddsUrl =  baseThreddsUrl + subdir + "/catalog.html"
		thredds_last := ThreddsDataDir + "/thredds_last"
		g, _ := os.Create(thredds_last)
		defer g.Close()
		g.WriteString(timestamp)
		thredds_nc := ThreddsDataDir + "/*.nc"
		rm_thredds_nc := "rm -f " + thredds_nc
		exec.Command("/bin/sh", "-c", rm_thredds_nc).CombinedOutput()
}
func setNewCookie (subdir string, timestamp string, w http.ResponseWriter){
		expiry_date := time.Date(2050, 12, 31, 23, 59, 59, 0, time.Local) // Saturday, December 31, 2050 at 11:59:59 PM
    	cookie_value := subdir + ":" + timestamp
		cookie := &http.Cookie{
			Name:  "Thredds_session",
			Value: cookie_value,
			Expires: expiry_date,
		}
		http.SetCookie(w, cookie)
}
func setNewTimeStamp(subdir string, timestamp string){
		ThreddsDataDir = baseThreddsDataDir + subdir
//fmt.Println(ThreddsDataDir)
		ThreddsUrl =  baseThreddsUrl + subdir + "/catalog.html"
		thredds_last := ThreddsDataDir + "/thredds_last"
		mkThreddsDataDir := "mkdir -p " + ThreddsDataDir
//fmt.Println(mkThreddsDataDir)
		exec.Command("/bin/sh", "-c", mkThreddsDataDir).CombinedOutput()
		f, _ := os.Create(thredds_last)

		defer f.Close()
		f.WriteString(timestamp)
}
func deleteNCs(subdir string) {
		ThreddsDataDir = baseThreddsDataDir + subdir
		thredds_nc := ThreddsDataDir + "/*.nc"
		rm_thredds_nc := "rm -f " + thredds_nc
		exec.Command("/bin/sh", "-c", rm_thredds_nc).CombinedOutput()
}
func Init_thredds(w http.ResponseWriter, r *http.Request) {
	Referer = ""
	referer := r.Header.Get("Referer")
    rr := regexp.MustCompile(`(?P<http>.*)://(?P<Domain>.*):(?P<Port>.*)/`)
    m := rr.FindStringSubmatch(referer)
    if (len(m) > 0) {
    	// This is a localhost:3001 call. Have a fixed subdir name
    	Referer = m[2]
    	if (m[2] == "localhost" || m[2] == "127.0.0.1"){
			ThreddsDataDir = baseThreddsDataDir + "localhost"
			ThreddsUrl =  baseThreddsUrl +  "localhost" + "/catalog.html"
			mkThreddsDataDir := "mkdir -p " + ThreddsDataDir
			exec.Command("/bin/sh", "-c", mkThreddsDataDir).CombinedOutput()
			AddToThredds = 1	
			return
    	}
    }
	// Get the cookie 
	c, err := r.Cookie("Thredds_session")

	// If no cookie has come, it does not mean that this is the first round.
	// Hence, check whether a subdir exists
	if ThreddsUrl == baseThreddsUrl {
//fmt.Printf("1. ThreddsUrl=%s; %s\n", ThreddsUrl, err) 
//	if err != nil && ThreddsUrl == baseThreddsUrl {
		if err != nil {
//fmt.Printf("2. ThreddsUrl=%s\n", ThreddsUrl) 
			// Create a 32-char alphanumeric random string as cookie value
			rand.Seed(time.Now().UnixNano())
			letterRunes := []rune("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
			rand_str := make([]rune, 32)  
			for i := range rand_str {
				rand_str[i] = letterRunes[rand.Intn(len(letterRunes))]
			}
			subdir := string(rand_str)
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			setNewCookie(subdir, timestamp, w)
			setNewTimeStamp(subdir, timestamp)
			deleteNCs(subdir)
		} else {
			s := strings.Split(c.Value, ":")
			subdir := s[0]
			ThreddsDataDir = baseThreddsDataDir + subdir
			ThreddsUrl =  baseThreddsUrl + subdir + "/catalog.html"
			c_timestamp := s[1]
			ThreddsDataDir = baseThreddsDataDir + subdir
			thredds_last := ThreddsDataDir + "/thredds_last"
			ft, _ := ioutil.ReadFile(thredds_last)
			f_timestamp := string(ft)
			if (c_timestamp == f_timestamp) {
				setNewTimestamp(subdir, w)
				deleteNCs(subdir)
				nc_added = 0
			}
		}
	}
}
func Delete_thredds_nc() {
    thredds_last := ThreddsDataDir + "/thredds_last"
    thredds_nc := ThreddsDataDir + "/*.nc"
	timestamp, _ := ioutil.ReadFile(thredds_last)
	timestamp_int, _ := strconv.Atoi(string(timestamp))
	et := time.Now().Unix() - int64(timestamp_int)
	/*
	How the set of NC files are created:
	 A single zoom action sends several http requests within 2 sec. Must not 
	 delete the NC files between those requests. If the last access was >= 2 sec, 
	 we can delete the previous NC links. There is a risk that they 
	 may not be deleted if the user consecutively zooms within 2 sec. 
	  
	 We cannot use cookies here to store the prev time, as a cookie gets set 
	 only AFTER all the calls end. Hence, using a local file to write the time. 
	 It is not ideal!
	*/
	if (et > 5) { // To be safe, using 5 sec.
		fmt.Println("-- Deleting the previous soft links...")		
		f, _ := os.Create(thredds_last)
		defer f.Close()
		now := strconv.FormatInt(time.Now().Unix(), 10)
		f.WriteString(now)
		rm_thredds_nc := "rm -f " + thredds_nc
		exec.Command("/bin/sh", "-c", rm_thredds_nc).CombinedOutput()
	}
}
func Add_thredds_nc (ds GDALDataset) {
// ds.DSName = NETCDF:"/g/data2/rs0/datacube/002/LS8_OLI_NBAR/-4_-35/LS8_OLI_NBAR_3577_-4_-35_2013_v1496403192.nc":red	
// split the above string into an array, m[]
// The regex is similar to that in Perl as e.g. m/(.*):"(.*)":(.*)/
//  where $1, $2, and $3 hold the values, the matched string within parenthesis go in as the array elements, m[0], m[1] and m[2]
// The names, <Type>, <File> and <NameSpace>, are for clarity alone. They are not used to address the array elements.
// (?P<Type>.*) = NETCDF 
// (?P<File>.*) = /g/data2/rs0/datacube/002/LS8_OLI_NBAR/-4_-35/LS8_OLI_NBAR_3577_-4_-35_2013_v1496403192.nc
// (?P<NameSpace>.*) = red
	r := regexp.MustCompile(`(?P<Type>.*):"(?P<File>.*)":(?P<NameSpace>.*)`)
    m := r.FindStringSubmatch(ds.DSName)
//fmt.Printf("**** %s\n", ds.DSName)
   	if (Referer == "localhost" || m[2] == "127.0.0.1"){
   		ThreddsDataDir = baseThreddsDataDir + "localhost"
	}
	s := strings.Split(m[2], "/")
	file := ThreddsDataDir + "/" + s[len(s)-1]
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		return
	}
    nc_added += 1
    fmt.Printf("%d. %s\n", nc_added, m[2])
	exec.Command("ln", "-s", m[2], ThreddsDataDir).CombinedOutput()
}
