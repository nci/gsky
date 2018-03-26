package main

import (
	extr "./extractor"
	"testing"
)

func stringIn(str string, strs []string) bool {
	for _, s := range strs {
		if str == s {
			return true
		}
	}
	return false
}

func checkGeoFile(test *testing.T, driver, path string, namespaces []string) {

	geo, err := extr.ExtractGDALInfo(path)
	if err != nil {
		test.Errorf("ExtractGDALInfo %s", path)
	}

	if geo.FileName != path {
		test.Errorf("GeoFile.FileName %s", path)
	}

	if geo.Driver != driver {
		test.Errorf("GeoFile.Driver %s", path)
	}

	for i, ds := range geo.DataSets {

		if !stringIn(ds.Type, []string{"Byte", "Int16", "UInt16", "Float32"}) {
			test.Errorf("[%d] invalid Type: %v", i, ds)
		}

		if len(ds.TimeStamps) == 0 {
			test.Errorf("[%d] missing TimeStamps: %v", i, ds)
		}

		if len(namespaces) > 0 && !stringIn(ds.NameSpace, namespaces) {
			test.Errorf("[%d] unexpected NameSpace: %v", i, ds)
		}

		if ds.Polygon == "" {
			test.Errorf("[%d] missing Polygon: %v", i, ds)
		}

		if ds.ProjWKT == "" {
			test.Errorf("[%d] missing ProjWKT: %v", i, ds)
		}
	}
}

func TestModisMCD43A4(test *testing.T) {
	checkGeoFile(test,
		`HDF4`,
		`/g/data2/u39/public/data/modis/lpdaac-tiles-c5/MCD43A4.005/2011.07.04/MCD43A4.A2011185.h17v01.005.2011215172011.hdf`,
		[]string{
			`Nadir_Reflectance_Band1`,
			`Nadir_Reflectance_Band2`,
			`Nadir_Reflectance_Band3`,
			`Nadir_Reflectance_Band4`,
			`Nadir_Reflectance_Band5`,
			`Nadir_Reflectance_Band6`,
			`Nadir_Reflectance_Band7`,
		},
	)
}

func TestModisMCD43A4frac(test *testing.T) {
	checkGeoFile(test,
		`netCDF`,
		`/g/data2/u39/public/prep/modis-fc/FC.v302.MCD43A4/FC.v302.MCD43A4.h00v08.2011.005.nc`,
		[]string{
			`phot_veg`,
			`nphot_veg`,
			`bare_soil`,
		},
	)
}

func TestCHRIPS2(test *testing.T) {
	checkGeoFile(test,
		`netCDF`,
		`/g/data1/sp9/CHIRPS-2.0/chirps-v2.0.1983.dekads.nc`,
		[]string{},
	)
}

func testDEA(test *testing.T, path string) {
	checkGeoFile(test,
		`netCDF`,
		path,
		[]string{
			"blue",
			"green",
			"red",
			"nir",
			"swir1",
			"swir2",
			"coastal_aerosol",
		},
	)
}

func TestDEA(test *testing.T) {
	testDEA(test, `/g/data2/rs0/datacube/002/LS5_TM_NBAR/22_-34/LS5_TM_NBAR_3577_22_-34_1991_v1496739620.nc`)
	testDEA(test, `/g/data2/rs0/datacube/002/LS5_TM_NBART/22_-34/LS5_TM_NBART_3577_22_-34_1995_v1498831246.nc`)
	testDEA(test, `/g/data2/rs0/datacube/002/LS7_ETM_NBAR/-6_-23/LS7_ETM_NBAR_3577_-6_-23_20160325013330500000.nc`)
	testDEA(test, `/g/data2/rs0/datacube/002/LS7_ETM_NBART/-6_-23/LS7_ETM_NBART_3577_-6_-23_20161026014014500000_v1508471591.nc`)
	testDEA(test, `/g/data2/rs0/datacube/002/LS8_OLI_NBAR/22_-34/LS8_OLI_NBAR_3577_22_-34_20170618233540000000_v1508439326.nc`)
	testDEA(test, `/g/data2/rs0/datacube/002/LS8_OLI_NBART/22_-34/LS8_OLI_NBART_3577_22_-34_20170704233543500000_v1508468655.nc`)
}

func TestHLTC(test *testing.T) {
	checkGeoFile(test,
		`netCDF`,
		`/g/data2/fk4/datacube/002/HLTC/HLTC_2_0/netcdf/COMPOSITE_LOW_88_143.71_-40.47_20050101_20170101_PER_20.nc`,
		[]string{
			"LOW:red",
			"LOW:green",
			"LOW:blue",
			"LOW:nir",
			"LOW:swir1",
			"LOW:swir2",
		},
	)
}

func TestITEM(test *testing.T) {
	checkGeoFile(test,
		`netCDF`,
		`/g/data2/fk4/datacube/002/ITEM/ITEM_2_0/netcdf/ITEM_REL_299_123.28_-15.24.nc`,
		[]string{
			"relative",
			"stddev",
		},
	)
}
