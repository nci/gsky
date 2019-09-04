package extractor

// #include <stdio.h>
// #include <stdlib.h>
// #include "gdal.h"
// #include "ogr_srs_api.h" /* for SRS calls */
// #include "cpl_string.h"
// #cgo pkg-config: gdal
//char *getProj4(char *projWKT)
//{
//	char *pszProj4;
//	char *result;
//	OGRSpatialReferenceH hSRS;
//
//	hSRS = OSRNewSpatialReference(projWKT);
//	OSRExportToProj4(hSRS, &pszProj4);
//	result = strdup(pszProj4);
//
//	OSRDestroySpatialReference(hSRS);
//	CPLFree(pszProj4);
//
//	return result;
//}
import "C"

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unsafe"
)

var LogErr *log.Logger

func init() {
	// By default, gdalinfo automatically saves an auxiliary xml file under the
	// same folder of the data file. This is problematic for us as the data files
	// we want to crawl are often owned by someone else.
	C.CPLSetConfigOption(C.CString("GDAL_PAM_ENABLED"), C.CString("NO"))
	C.GDALAllRegister()

	LogErr = log.New(os.Stderr, "Crawler: ", log.Ldate|log.Ltime|log.Lshortfile)
}

var dateFormats []string = []string{"2006-01-02 15:04:05.0", "2006-1-2 15:4:5"}
var durationUnits map[string]time.Duration = map[string]time.Duration{"seconds": time.Second, "hours": time.Hour, "days": time.Hour * 24}
var CsubDS *C.char = C.CString("SUBDATASETS")
var CncVarname *C.char = C.CString("NETCDF_VARNAME")

var CncExtraDims *C.char = C.CString("NETCDF_DIM_EXTRA")

func ExtractGDALInfo(path string, concLimit int, approx bool, config *Config) (*GeoFile, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	hDataset := C.GDALOpen(cPath, C.GA_ReadOnly)
	if hDataset == nil {
		err := C.CPLGetLastErrorMsg()
		return &GeoFile{}, fmt.Errorf("%v", C.GoString(err))
	}
	defer C.GDALClose(hDataset)

	hDriver := C.GDALGetDatasetDriver(hDataset)
	cShortName := C.GDALGetDriverShortName(hDriver)
	shortName := C.GoString(cShortName)

	mObj := C.GDALMajorObjectH(hDataset)
	metadata := C.GDALGetMetadata(mObj, CsubDS)
	nsubds := C.CSLCount(metadata) / C.int(2)

	var datasets = []*GeoMetaData{}
	if nsubds == C.int(0) {
		// There are no subdatasets
		dsInfo, err := getDataSetInfo(path, cPath, shortName, approx, config)
		if err != nil {
			LogErr.Printf("%v", err)
			return &GeoFile{}, fmt.Errorf("%v", err)
		}
		datasets = append(datasets, dsInfo)

	} else {
		// There are subdatasets
		tmpDatasets := make([]*GeoMetaData, int(nsubds))

		var wg sync.WaitGroup
		wg.Add(int(nsubds))

		if concLimit > int(nsubds) {
			concLimit = int(nsubds)
		}
		concPool := make(chan int, concLimit)
		for i := C.int(1); i <= nsubds; i++ {
			concPool <- 1
			go func(ds []*GeoMetaData, isub int) {
				defer wg.Done()
				subDSId := C.CString(fmt.Sprintf("SUBDATASET_%d_NAME", isub))
				pszSubdatasetName := C.CSLFetchNameValue(metadata, subDSId)
				dsInfo, err := getDataSetInfo(path, pszSubdatasetName, shortName, approx, config)

				<-concPool
				if err == nil {
					ds[isub-1] = dsInfo
				} else {
					LogErr.Printf("%v", err)
				}
			}(tmpDatasets, int(i))
		}

		wg.Wait()

		for _, ds := range tmpDatasets {
			if ds != nil {
				datasets = append(datasets, ds)
			}
		}
	}

	return &GeoFile{FileName: path, Driver: shortName, DataSets: datasets}, nil
}

func getDataSetInfo(filename string, dsName *C.char, driverName string, approx bool, config *Config) (*GeoMetaData, error) {
	datasetName := C.GoString(dsName)
	hSubdataset := C.GDALOpen(dsName, C.GDAL_OF_READONLY)
	if hSubdataset == nil {
		return &GeoMetaData{}, fmt.Errorf("GDAL could not open dataset: %s", datasetName)
	}
	defer C.GDALClose(hSubdataset)

	ruleSet, nameFields, timeStamp := parseName(filename, config)

	var ncTimes []string
	var err error
	var times []time.Time
	if ruleSet.NcMetadata || driverName == "netCDF" || driverName == "JP2OpenJPEG" {
		ncTimes, err = getNCTime(datasetName, hSubdataset, ruleSet)
		if err != nil && timeStamp.IsZero() && len(ruleSet.TimesText) == 0 {
			return &GeoMetaData{}, fmt.Errorf("Error parsing dates: %v", err)
		}
	}

	if err == nil && ncTimes != nil {
		for _, timestr := range ncTimes {
			t, err := time.ParseInLocation("2006-01-02T15:04:05Z", timestr, time.UTC)
			if err != nil {
				log.Println(err)
				continue
			}
			times = append(times, t)
		}
	} else if len(ruleSet.TimesText) > 0 {
		for _, ts := range ruleSet.TimesText {
			t, err := time.ParseInLocation("2006-01-02T15:04:05Z", ts, time.UTC)
			if err != nil {
				log.Println(err)
				continue
			}
			times = append(times, t)
		}
	} else {
		times = append(times, timeStamp)
	}

	var ncAxes []*DatasetAxis
	if ruleSet.NcMetadata || driverName == "netCDF" || driverName == "JP2OpenJPEG" {
		if ruleSet.TimeAxis != nil {
			if len(ruleSet.TimeAxis.Shape) == 0 || ruleSet.TimeAxis.Shape[0] < 0 {
				ruleSet.TimeAxis.Shape = []int{len(times)}
			}

			if len(ruleSet.TimeAxis.Strides) == 0 || ruleSet.TimeAxis.Strides[0] < 0 {
				ruleSet.TimeAxis.Shape = []int{1}
			}

		}
		ncAxes, err = getNCAxes(datasetName, hSubdataset, ruleSet)
	}

	for _, axis := range ruleSet.AxesText {
		foundAxis := false
		for ia, ax := range ncAxes {
			if ax.Name == axis.Name {
				foundAxis = true
				ncAxes[ia] = axis
				break
			}
		}

		if !foundAxis {
			ncAxes = append(ncAxes, axis)
		}
	}

	var geoLocation *GeoLocInfo
	if ruleSet.GeoLoc != nil {
		geoLocTmp, err := getGeoLocation(ruleSet.GeoLoc, filename)
		if err != nil {
			log.Println(err)
		} else {
			geoLocation = geoLocTmp
		}
	}

	hBand := C.GDALGetRasterBand(hSubdataset, 1)
	nOvr := C.GDALGetOverviewCount(hBand)
	ovrs := make([]*Overview, int(nOvr))
	for i := C.int(0); i < nOvr; i++ {
		hOvr := C.GDALGetOverview(hBand, i)
		ovrs[int(i)] = &Overview{XSize: int32(C.GDALGetRasterBandXSize(hOvr)), YSize: int32(C.GDALGetRasterBandYSize(hOvr))}
	}

	projWkt := C.GoString(C.GDALGetProjectionRef(hSubdataset))

	if projWkt == "" || ruleSet.SRSText != SRSDetect {
		projWkt = ruleSet.SRSText
	}
	cProjWKT := C.CString(projWkt)

	cProj4 := C.getProj4(cProjWKT)
	C.free(unsafe.Pointer(cProjWKT))
	proj4 := C.GoString(cProj4)
	C.free(unsafe.Pointer(cProj4))

	if proj4 == "" || ruleSet.Proj4Text != Proj4Detect {
		proj4 = ruleSet.Proj4Text
	}

	var nameSpace string

	nsPath := nameFields["namespace"]

	// GDAL dataset string is dependent on the driver, example:
	// NETCDF:"/g/data2/fk4/datacube/002/HLTC/HLTC_2_0/netcdf/COMPOSITE_HIGH_100_146.84_-40.8_20000101_20170101_PER_20.nc":blue
	nsDataset := func() (ns string) {
		parts := strings.Split(datasetName, ":")
		if len(parts) > 2 {
			ns = parts[len(parts)-1]
		}
		return
	}()

	switch ruleSet.NameSpace {

	case NSCombine:
		nameSpace = fmt.Sprintf("%s:%s", nsPath, nsDataset)

	case NSPath:
		nameSpace = nsPath

	case NSDataset:
		nameSpace = nsDataset

	}

	dArr := [6]C.double{}
	C.GDALGetGeoTransform(hSubdataset, &dArr[0])

	geot := (*[6]float64)(unsafe.Pointer(&dArr))[:]
	polyWkt := getGeometryWKT(geot, int(C.GDALGetRasterXSize(hSubdataset)), int(C.GDALGetRasterYSize(hSubdataset)), ruleSet)

	noData := C.GDALGetRasterNoDataValue(hBand, nil)

	var mins, maxs, means, stddevs []float64
	var sampleCounts []int

	dsWidth := int(C.GDALGetRasterXSize(hSubdataset))
	dsHeight := int(C.GDALGetRasterYSize(hSubdataset))

	if ruleSet.ComputeStats {
		bandSize := dsWidth * dsHeight

		nBands := C.GDALGetRasterCount(hSubdataset)
		for ib := 1; ib <= int(nBands); ib++ {
			hBand := C.GDALGetRasterBand(hSubdataset, C.int(ib))

			approxC := C.int(1)
			if !approx {
				approxC = C.int(0)
			}

			stats := [4]C.double{} // min, max, mean, stddev
			cErr := C.GDALGetRasterStatistics(hBand, approxC, C.int(1), &stats[0], &stats[1], &stats[2], &stats[3])
			if cErr != C.CE_None {
				for i := 0; i < len(stats); i++ {
					stats[i] = noData
				}
			}

			validPercent := -1.0
			validPercentC := C.GDALGetMetadataItem(C.GDALMajorObjectH(hBand), C.CString("STATISTICS_VALID_PERCENT"), nil)
			if validPercentC != nil {
				if per, e := strconv.ParseFloat(C.GoString(validPercentC), 64); e == nil {
					validPercent = per
				}
			}

			sampleCount := -1
			if validPercent >= 0 {
				sampleCount = int(float64(bandSize) * validPercent)
			}

			mins = append(mins, float64(stats[0]))
			maxs = append(maxs, float64(stats[1]))
			means = append(means, float64(stats[2]))
			stddevs = append(stddevs, float64(stats[3]))
			sampleCounts = append(sampleCounts, sampleCount)
		}
	}

	return &GeoMetaData{
		DataSetName:  datasetName,
		NameSpace:    nameSpace,
		Type:         C.GoString(C.GDALGetDataTypeName(C.GDALGetRasterDataType(hBand))),
		RasterCount:  int32(C.GDALGetRasterCount(hSubdataset)),
		TimeStamps:   times,
		XSize:        int32(dsWidth),
		YSize:        int32(dsHeight),
		Polygon:      polyWkt,
		ProjWKT:      projWkt,
		Proj4:        proj4,
		GeoTransform: geot,
		Overviews:    ovrs,
		Mins:         mins,
		Maxs:         maxs,
		Means:        means,
		StdDevs:      stddevs,
		SampleCounts: sampleCounts,
		NoData:       float64(noData),
		Axes:         ncAxes,
		GeoLocation:  geoLocation,
	}, nil
}

func getGeometryWKT(geot []float64, xSize, ySize int, ruleSet *RuleSet) string {
	var ulX, ulY, lrX, lrY C.double

	if len(ruleSet.BBox) != 4 {
		C.GDALApplyGeoTransform((*C.double)(unsafe.Pointer(&geot[0])), 0, 0, &ulX, &ulY)
		C.GDALApplyGeoTransform((*C.double)(unsafe.Pointer(&geot[0])), C.double(xSize), C.double(ySize), &lrX, &lrY)
	} else {
		ulX = C.double(ruleSet.BBox[0])
		ulY = C.double(ruleSet.BBox[1])
		lrX = C.double(ruleSet.BBox[2])
		lrY = C.double(ruleSet.BBox[3])
		geot[0] = ruleSet.BBox[0]
		geot[3] = ruleSet.BBox[1]
	}
	return fmt.Sprintf("POLYGON ((%f %f,%f %f,%f %f,%f %f,%f %f))", ulX, ulY, ulX, lrY, lrX, lrY, lrX, ulY, ulX, ulY)
}

func parseName(path string, config *Config) (*RuleSet, map[string]string, time.Time) {
	_, basename := filepath.Split(path)

	if len(config.RuleSets) > 0 {
		CollectionRuleSets = config.RuleSets
	}

	for _, ruleSet := range CollectionRuleSets {
		re := regexp.MustCompile(ruleSet.Pattern)

		fname := basename
		if ruleSet.MatchFullPath {
			fname = path
		}
		if re.MatchString(fname) {
			match := re.FindStringSubmatch(fname)

			result := make(map[string]string)
			for i, name := range re.SubexpNames() {
				if i != 0 {
					result[name] = match[i]
				}
			}
			return &ruleSet, result, parseTime(result)
		}
	}
	return nil, nil, time.Time{}
}

func getRegexMatches(source string, pattern string) map[string]string {
	result := make(map[string]string)
	re := regexp.MustCompile(pattern)
	if re.MatchString(source) {
		match := re.FindStringSubmatch(source)
		for i, name := range re.SubexpNames() {
			if i != 0 {
				result[name] = match[i]
			}
		}
	}

	return result
}

func instantiateTemplate(tplText string, data interface{}) (string, error) {
	tpl, err := template.New("template").Parse(tplText)
	if err != nil {
		return "", fmt.Errorf("Error trying to parse template document: %v", err)
	}

	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("Error executing template: %v\n", err)
	}

	return buf.String(), nil

}

func getGeoLocation(geoLoc *GeoLocRule, path string) (*GeoLocInfo, error) {
	xMatches := getRegexMatches(path, geoLoc.XDatasetPattern)
	xDataset, err := instantiateTemplate(geoLoc.XDatasetTemplate, xMatches)
	if err != nil {
		return nil, err
	}

	yMatches := getRegexMatches(path, geoLoc.YDatasetPattern)
	yDataset, err := instantiateTemplate(geoLoc.YDatasetTemplate, yMatches)
	if err != nil {
		return nil, err
	}

	locInfo := &GeoLocInfo{XDataSetName: xDataset, XBand: 1, YDataSetName: yDataset, YBand: 1, LineOffset: 0, PixelOffset: 0, LineStep: 1, PixelStep: 1}

	if geoLoc.XBand != nil {
		locInfo.XBand = *geoLoc.XBand
	}

	if geoLoc.YBand != nil {
		locInfo.YBand = *geoLoc.YBand
	}

	if geoLoc.LineOffset != nil {
		locInfo.LineOffset = *geoLoc.LineOffset
	}

	if geoLoc.PixelOffset != nil {
		locInfo.PixelOffset = *geoLoc.PixelOffset
	}

	if geoLoc.LineStep != nil {
		locInfo.LineStep = *geoLoc.LineStep
	}

	if geoLoc.PixelStep != nil {
		locInfo.PixelStep = *geoLoc.PixelStep
	}

	return locInfo, nil
}

func parseTime(nameFields map[string]string) time.Time {
	if _, ok := nameFields["year"]; ok {
		year, _ := strconv.Atoi(nameFields["year"])
		t := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)

		if _, ok := nameFields["julian_day"]; ok {
			julianDay, _ := strconv.Atoi(nameFields["julian_day"])
			t = t.Add(time.Hour * 24 * time.Duration(julianDay-1))
		}

		if _, ok := nameFields["month"]; ok {
			if _, ok := nameFields["day"]; ok {
				month, _ := strconv.Atoi(nameFields["month"])
				day, _ := strconv.Atoi(nameFields["day"])
				t = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			}
		}

		if _, ok := nameFields["hour"]; ok {
			hour, _ := strconv.Atoi(nameFields["hour"])
			t = t.Add(time.Hour * time.Duration(hour))
		}

		if _, ok := nameFields["minute"]; ok {
			minute, _ := strconv.Atoi(nameFields["minute"])
			t = t.Add(time.Minute * time.Duration(minute))
		}

		if _, ok := nameFields["second"]; ok {
			second, _ := strconv.Atoi(nameFields["second"])
			t = t.Add(time.Second * time.Duration(second))
		}
		return t
	}
	return time.Time{}
}

func getDate(inDate string) (time.Time, error) {
	for _, dateFormat := range dateFormats {
		if t, err := time.Parse(dateFormat, inDate); err == nil {
			return t, err
		}
	}
	return time.Time{}, fmt.Errorf("Could not parse time string: %s", inDate)
}

func getNCTime(sdsName string, hSubdataset C.GDALDatasetH, ruleSet *RuleSet) ([]string, error) {
	times := []string{}
	mObj := C.GDALMajorObjectH(hSubdataset)
	metadata := C.GDALGetMetadata(mObj, nil)

	timeDim := "time"
	if ruleSet.TimeAxis != nil && len(ruleSet.TimeAxis.Name) > 0 && ruleSet.TimeAxis.Name != "time" {
		timeDim = ruleSet.TimeAxis.Name
	}

	timeUnits := ruleSet.TimeUnits
	if len(timeUnits) == 0 {
		CtimeUnits := C.CString(fmt.Sprintf("%s#units", timeDim))
		defer C.free(unsafe.Pointer(CtimeUnits))

		idx := C.CSLFindName(metadata, CtimeUnits)
		if idx == -1 {
			return nil, fmt.Errorf("Does not contain timeUnits string")
		}

		timeUnits = C.GoString(C.CSLFetchNameValue(metadata, CtimeUnits))
	}
	timeUnitsWords := strings.Split(timeUnits, " ")
	if timeUnitsWords[1] != "since" {
		return nil, fmt.Errorf("Cannot parse Units string")
	}
	if len(timeUnitsWords) == 3 {
		timeUnitsWords = append(timeUnitsWords, "00:00:00.0")
	}
	stepUnitStr := strings.Trim(timeUnitsWords[0], " ")
	stepUnit := durationUnits[stepUnitStr]
	startDate, err := getDate(strings.Join(timeUnitsWords[2:], " "))
	if err != nil {
		return times, err
	}

	CncDimTimeValues := C.CString(fmt.Sprintf("NETCDF_DIM_%s_VALUES", timeDim))
	defer C.free(unsafe.Pointer(CncDimTimeValues))

	value := C.CSLFetchNameValue(metadata, CncDimTimeValues)
	if value != nil {

		timeStr := C.GoString(value)
		for _, tStr := range strings.Split(strings.Trim(timeStr, "{}"), ",") {
			tF, err := strconv.ParseFloat(tStr, 64)
			if err != nil {
				return times, fmt.Errorf("Problem parsing dates with dataset %s", sdsName)
			}
			d, f := math.Modf(tF)
			var t time.Time
			if stepUnitStr == "days" {
				t = startDate.AddDate(0, 0, int(d)).Add(time.Duration(f * float64(durationUnits["days"])))
			} else if stepUnitStr == "months" {
				t = startDate.AddDate(0, int(d), 0)
			} else if stepUnitStr == "years" {
				t = startDate.AddDate(int(d), 0, 0)
			} else {
				t = startDate.Add(time.Duration(d) * stepUnit)
			}
			times = append(times, t.Format("2006-01-02T15:04:05Z"))
		}

		return times, nil
	}
	return times, fmt.Errorf("Dataset %s doesn't contain times", sdsName)
}

func getNCAxes(sdsName string, hSubdataset C.GDALDatasetH, ruleSet *RuleSet) ([]*DatasetAxis, error) {
	var axes []*DatasetAxis
	mObj := C.GDALMajorObjectH(hSubdataset)
	metadata := C.GDALGetMetadata(mObj, nil)

	value := C.CSLFetchNameValue(metadata, CncExtraDims)
	if value == nil {
		return axes, fmt.Errorf("Failed to parse dimensions: %v", sdsName)
	}

	foundTimeAxis := false
	dimsStr := C.GoString(value)
	for _, dim := range strings.Split(strings.Trim(dimsStr, "{}"), ",") {
		dimValsField := fmt.Sprintf("NETCDF_DIM_%s_VALUES", dim)

		dimValsFieldC := C.CString(dimValsField)
		dimValsStrC := C.CSLFetchNameValue(metadata, dimValsFieldC)
		C.free(unsafe.Pointer(dimValsFieldC))

		dimValsStr := C.GoString(dimValsStrC)
		dimValues := strings.Split(strings.Trim(dimValsStr, "{}"), ",")

		if dim == "time" || (ruleSet.TimeAxis != nil && dim == ruleSet.TimeAxis.Name) {
			axis := &DatasetAxis{Name: "time", Shape: []int{len(dimValues)}, Grid: "default"}
			axes = append(axes, axis)
			foundTimeAxis = true
			continue
		}

		axis := &DatasetAxis{Name: dim, Shape: []int{len(dimValues)}, Grid: "enum"}

		for iv, val := range dimValues {
			vFloat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse the values of %v: %v, err pos: %d", dimsStr, dimValsStr, iv)
			}

			axis.Params = append(axis.Params, vFloat)
		}

		axes = append(axes, axis)
	}

	if !foundTimeAxis && ruleSet.TimeAxis != nil {
		axis := &DatasetAxis{Name: "time", Shape: ruleSet.TimeAxis.Shape, Strides: ruleSet.TimeAxis.Strides, Grid: "default"}
		axes = append(axes, axis)
	}

	if len(axes[len(axes)-1].Strides) == 0 {
		axes[len(axes)-1].Strides = []int{1}
	}

	accumStrides := 1
	for i := len(axes) - 2; i >= 0; i-- {
		accumStrides *= axes[i+1].Shape[0]
		axes[i].Strides = []int{accumStrides}
	}

	return axes, nil
}
