GSKY High Performance Crawler
=============================
Inputs
------
The main script to run to crawl data files is `crawl_pipeline.sh`. The inputs to the crawler are passed via environment variables. Such a design choice is to facilitate running the crawler as batch processing jobs in an HPC environment. One example is the PBSPro HPC environment at NCI.
1. `$CRAWL_FILE_LIST`: A list of files to crawl
2. `$CRAWL_DIR`: Instead of a user-supplied crawl file list, one can specify a root directory to crawl recursively.
3. `$CRAWL_PATTERN`: The pattern to match the files to be crawled. The pattrn syntax is the same as the one used by the `find` command. The default value is `*.nc` to crawl netCDF files.
4. `$CRAW_PARAMS`: These are additonal parameters for the `find` command. For exmple, one can specify `-mtime 1` to look for the modified files within last 24 hours. The default value is empty string.
5. `$CRAWL_CONC_LIMIT`: The number of crawler processes run in parrallel. The default value is 16. 

Outputs
-------
The crawler outputs a gzip compressed tsv file with extension of .tsv.gz. Each line consists of three fields as follows:
```
| field | description |
|---|---|
| full path | /g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc |
| metadata type | gdal |
| JSON blob | {"filename":"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc","file_type":"netCDF","geo_metadata":[....]} | 
```

### Notes:
* metadata type is just tag. If the metadata is intended for GSKY, the tag is gdal
* The JSON blob can be of any structure and depth. MAS uses Postgres JSON functions to extract fields, generating materialized views for the RESTful API.

A full example
--------------
File to be crawled: `/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc`

```json
{
   "filename":"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc",
   "file_type":"netCDF",
   "geo_metadata":[
      {
         "ds_name":"NETCDF:\"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc\":blue",
         "namespace":"HIGH:blue",
         "array_type":"Float32",
         "raster_count":1,
         "timestamps":[
            "2016-10-31T23:59:59Z"
         ],
         "x_size":7340,
         "y_size":5196,
         "geotransform":[
            1381675,
            25,
            0,
            -1738000,
            0,
            -25
         ],
         "polygon":"POLYGON ((1381675.000000 -1738000.000000,1381675.000000 -1867900.000000,1565175.000000 -1867900.000000,1565175.000000 -1738000.000000,1381675.000000 -1738000.000000))",
         "proj_wkt":"PROJCS[\"GDA94 / Australian Albers\",GEOGCS[\"GDA94\",DATUM[\"Geocentric_Datum_of_Australia_1994\",SPHEROID[\"GRS 1980\",6378137,298.257222101,AUTHORITY[\"EPSG\",\"7019\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6283\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9122\"]],AUTHORITY[\"EPSG\",\"4283\"]],PROJECTION[\"Albers_Conic_Equal_Area\"],PARAMETER[\"standard_parallel_1\",-18],PARAMETER[\"standard_parallel_2\",-36],PARAMETER[\"latitude_of_center\",0],PARAMETER[\"longitude_of_center\",132],PARAMETER[\"false_easting\",0],PARAMETER[\"false_northing\",0],UNIT[\"metre\",1,AUTHORITY[\"EPSG\",\"9001\"]],AXIS[\"Easting\",EAST],AXIS[\"Northing\",NORTH],AUTHORITY[\"EPSG\",\"3577\"]]",
         "proj4":"+proj=aea +lat_1=-18 +lat_2=-36 +lat_0=0 +lon_0=132 +x_0=0 +y_0=0 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs "
      },
      {
         "ds_name":"NETCDF:\"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc\":green",
         "namespace":"HIGH:green",
         "array_type":"Float32",
         "raster_count":1,
         "timestamps":[
            "2016-10-31T23:59:59Z"
         ],
         "x_size":7340,
         "y_size":5196,
         "geotransform":[
            1381675,
            25,
            0,
            -1738000,
            0,
            -25
         ],
         "polygon":"POLYGON ((1381675.000000 -1738000.000000,1381675.000000 -1867900.000000,1565175.000000 -1867900.000000,1565175.000000 -1738000.000000,1381675.000000 -1738000.000000))",
         "proj_wkt":"PROJCS[\"GDA94 / Australian Albers\",GEOGCS[\"GDA94\",DATUM[\"Geocentric_Datum_of_Australia_1994\",SPHEROID[\"GRS 1980\",6378137,298.257222101,AUTHORITY[\"EPSG\",\"7019\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6283\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9122\"]],AUTHORITY[\"EPSG\",\"4283\"]],PROJECTION[\"Albers_Conic_Equal_Area\"],PARAMETER[\"standard_parallel_1\",-18],PARAMETER[\"standard_parallel_2\",-36],PARAMETER[\"latitude_of_center\",0],PARAMETER[\"longitude_of_center\",132],PARAMETER[\"false_easting\",0],PARAMETER[\"false_northing\",0],UNIT[\"metre\",1,AUTHORITY[\"EPSG\",\"9001\"]],AXIS[\"Easting\",EAST],AXIS[\"Northing\",NORTH],AUTHORITY[\"EPSG\",\"3577\"]]",
         "proj4":"+proj=aea +lat_1=-18 +lat_2=-36 +lat_0=0 +lon_0=132 +x_0=0 +y_0=0 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs "
      },
      {
         "ds_name":"NETCDF:\"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc\":red",
         "namespace":"HIGH:red",
         "array_type":"Float32",
         "raster_count":1,
         "timestamps":[
            "2016-10-31T23:59:59Z"
         ],
         "x_size":7340,
         "y_size":5196,
         "geotransform":[
            1381675,
            25,
            0,
            -1738000,
            0,
            -25
         ],
         "polygon":"POLYGON ((1381675.000000 -1738000.000000,1381675.000000 -1867900.000000,1565175.000000 -1867900.000000,1565175.000000 -1738000.000000,1381675.000000 -1738000.000000))",
         "proj_wkt":"PROJCS[\"GDA94 / Australian Albers\",GEOGCS[\"GDA94\",DATUM[\"Geocentric_Datum_of_Australia_1994\",SPHEROID[\"GRS 1980\",6378137,298.257222101,AUTHORITY[\"EPSG\",\"7019\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6283\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9122\"]],AUTHORITY[\"EPSG\",\"4283\"]],PROJECTION[\"Albers_Conic_Equal_Area\"],PARAMETER[\"standard_parallel_1\",-18],PARAMETER[\"standard_parallel_2\",-36],PARAMETER[\"latitude_of_center\",0],PARAMETER[\"longitude_of_center\",132],PARAMETER[\"false_easting\",0],PARAMETER[\"false_northing\",0],UNIT[\"metre\",1,AUTHORITY[\"EPSG\",\"9001\"]],AXIS[\"Easting\",EAST],AXIS[\"Northing\",NORTH],AUTHORITY[\"EPSG\",\"3577\"]]",
         "proj4":"+proj=aea +lat_1=-18 +lat_2=-36 +lat_0=0 +lon_0=132 +x_0=0 +y_0=0 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs "
      },
      {
         "ds_name":"NETCDF:\"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc\":nir",
         "namespace":"HIGH:nir",
         "array_type":"Float32",
         "raster_count":1,
         "timestamps":[
            "2016-10-31T23:59:59Z"
         ],
         "x_size":7340,
         "y_size":5196,
         "geotransform":[
            1381675,
            25,
            0,
            -1738000,
            0,
            -25
         ],
         "polygon":"POLYGON ((1381675.000000 -1738000.000000,1381675.000000 -1867900.000000,1565175.000000 -1867900.000000,1565175.000000 -1738000.000000,1381675.000000 -1738000.000000))",
         "proj_wkt":"PROJCS[\"GDA94 / Australian Albers\",GEOGCS[\"GDA94\",DATUM[\"Geocentric_Datum_of_Australia_1994\",SPHEROID[\"GRS 1980\",6378137,298.257222101,AUTHORITY[\"EPSG\",\"7019\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6283\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9122\"]],AUTHORITY[\"EPSG\",\"4283\"]],PROJECTION[\"Albers_Conic_Equal_Area\"],PARAMETER[\"standard_parallel_1\",-18],PARAMETER[\"standard_parallel_2\",-36],PARAMETER[\"latitude_of_center\",0],PARAMETER[\"longitude_of_center\",132],PARAMETER[\"false_easting\",0],PARAMETER[\"false_northing\",0],UNIT[\"metre\",1,AUTHORITY[\"EPSG\",\"9001\"]],AXIS[\"Easting\",EAST],AXIS[\"Northing\",NORTH],AUTHORITY[\"EPSG\",\"3577\"]]",
         "proj4":"+proj=aea +lat_1=-18 +lat_2=-36 +lat_0=0 +lon_0=132 +x_0=0 +y_0=0 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs "
      },
      {
         "ds_name":"NETCDF:\"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc\":swir1",
         "namespace":"HIGH:swir1",
         "array_type":"Float32",
         "raster_count":1,
         "timestamps":[
            "2016-10-31T23:59:59Z"
         ],
         "x_size":7340,
         "y_size":5196,
         "geotransform":[
            1381675,
            25,
            0,
            -1738000,
            0,
            -25
         ],
         "polygon":"POLYGON ((1381675.000000 -1738000.000000,1381675.000000 -1867900.000000,1565175.000000 -1867900.000000,1565175.000000 -1738000.000000,1381675.000000 -1738000.000000))",
         "proj_wkt":"PROJCS[\"GDA94 / Australian Albers\",GEOGCS[\"GDA94\",DATUM[\"Geocentric_Datum_of_Australia_1994\",SPHEROID[\"GRS 1980\",6378137,298.257222101,AUTHORITY[\"EPSG\",\"7019\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6283\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9122\"]],AUTHORITY[\"EPSG\",\"4283\"]],PROJECTION[\"Albers_Conic_Equal_Area\"],PARAMETER[\"standard_parallel_1\",-18],PARAMETER[\"standard_parallel_2\",-36],PARAMETER[\"latitude_of_center\",0],PARAMETER[\"longitude_of_center\",132],PARAMETER[\"false_easting\",0],PARAMETER[\"false_northing\",0],UNIT[\"metre\",1,AUTHORITY[\"EPSG\",\"9001\"]],AXIS[\"Easting\",EAST],AXIS[\"Northing\",NORTH],AUTHORITY[\"EPSG\",\"3577\"]]",
         "proj4":"+proj=aea +lat_1=-18 +lat_2=-36 +lat_0=0 +lon_0=132 +x_0=0 +y_0=0 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs "
      },
      {
         "ds_name":"NETCDF:\"/g/data3/fr5/HLTC/COMPOSITE_HIGH_12_145.93_-16.43_19950101_20170101_PER_20.nc\":swir2",
         "namespace":"HIGH:swir2",
         "array_type":"Float32",
         "raster_count":1,
         "timestamps":[
            "2016-10-31T23:59:59Z"
         ],
         "x_size":7340,
         "y_size":5196,
         "geotransform":[
            1381675,
            25,
            0,
            -1738000,
            0,
            -25
         ],
         "polygon":"POLYGON ((1381675.000000 -1738000.000000,1381675.000000 -1867900.000000,1565175.000000 -1867900.000000,1565175.000000 -1738000.000000,1381675.000000 -1738000.000000))",
         "proj_wkt":"PROJCS[\"GDA94 / Australian Albers\",GEOGCS[\"GDA94\",DATUM[\"Geocentric_Datum_of_Australia_1994\",SPHEROID[\"GRS 1980\",6378137,298.257222101,AUTHORITY[\"EPSG\",\"7019\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6283\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9122\"]],AUTHORITY[\"EPSG\",\"4283\"]],PROJECTION[\"Albers_Conic_Equal_Area\"],PARAMETER[\"standard_parallel_1\",-18],PARAMETER[\"standard_parallel_2\",-36],PARAMETER[\"latitude_of_center\",0],PARAMETER[\"longitude_of_center\",132],PARAMETER[\"false_easting\",0],PARAMETER[\"false_northing\",0],UNIT[\"metre\",1,AUTHORITY[\"EPSG\",\"9001\"]],AXIS[\"Easting\",EAST],AXIS[\"Northing\",NORTH],AUTHORITY[\"EPSG\",\"3577\"]]",
         "proj4":"+proj=aea +lat_1=-18 +lat_2=-36 +lat_0=0 +lon_0=132 +x_0=0 +y_0=0 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs "
      }
   ]
}
```

| field | description |
|---|---|
| geo_metadata | An array of GDAL "datasets" for this file. |
| ds_name | A GDAL "dataset" string in a format relevant to the driver in use. GDAL uses this to locate and open files, not just the raw POSIX path. In this case the NetCDF driver has exposed the full path and a variable name. |
| namespace | A generic tag used by GSKY and MAS to group records into layers. For projects with nicely structured NetCDF files namespace may simply be a variable name, while for poorly curated projects it may come from a regular expression match on the path or something else entirely. |
| timestamps | For data with a time series component, an array of timestamps available in this file. As with namespace this depends on the project in question: it may come from file headers, or the path, or anywhere.
| polygon | Well known text of the bounding polygon for this file. |
