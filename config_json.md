# GSKY configuration file description

GSKY can currently provide WMS, WCS and WPS services (as defined by the
Open Geosaptial Consortium or OGC). The GSKY configuration file is a
JSON document (with the filename `config.json`). It specifies the
datasets and configuration parameters that GSKY needs to expose the
data layers and services.

This document describes the structure and contents of the
configuration file.  New datasets can be added to this file so that
GSKY can expose new layers and services.

## Configuration file high level structure

The configuration file is a JSON document with the following
structure:

```
{  
   "service_config": {  
      "ows_hostname": "gsky.example.com",
      "mas_address": "MAS_IP:80",
      "worker_nodes": [  
         "Worker_1_IP:6000",
         ...
         "Worker_n_IP:6000",
      ]
   },
   "layers": [  
        // List of WMS Layers
   ],
   "processes": [  
        // List of WPS Services
   ]
}
```

This document contains three keys at the top level:

* `service_config`: Provides information about the fully-qualified
  domain name associated with the instance, MAS RESTful API endpoint
  and the list of worker nodes used to process the data.

* `layers`: This field corresponds to the list of WMS layers
  exposed by GSKY. The structure of the documents defining the
  different layers is covered in the next section of this document.

* `processes`: This field corresponds to the list of WPS services
  exposed by GSKY. This functionality is currently implemented for a
  specific case and there is not a generic way of defining new
  services. This part is not documented as the interface needs to be
  redefined to be more generic.

## WMS layers

A WMS layer is defined using a JSON document specifying values used
internally by GSKY to locate and process the data as well as
parameters used to describe the layer in the WMS GetCapabilities
response. The WCS services provided by GSKY shares a major portion
of the processing pipeline as WMS does. Therefore, WCS inherits all
the WMS layer configurations. WCS, however, delivers raw data to the
client side. Thus the `offset_value`, `clip_value`, `scale_value`
and `colour palette` fields are not used by WCS.

A skeleton of the configuration of a WMS layer is as follows:

```
{  
   "name": "Name of the layer",
   "title": "Title of the layer (<title> in WMS GetCapabilities)",
   "abstract": "Abstract of the layer (<abstract> in WMS GetCapabilities)",
   "data_source": "/path/to/data",
   "start_isodate": "YYYY-MM-DDTHH:MM:SS.000Z",
   "end_isodate": "YYYY-MM-DDTHH:MM:SS.000Z",
   "step_days": int,
   "step_hours": int,
   "step_minutes": int,
   "accum": [true, false],
   "time_generator": ["regular", "mcd43", "chirps20", "monthly", "yearly", "mas"],
   "rgb_products": [],
   "offset_value": float64,
   "clip_value": float64,
   "scale_value": float64,
   "legend_path": "path to image with legend",
   "zoom_limit": float64,
   "palette": {
      "colours": [
         { "R": 215, "G": 25, "B": 28, "A": 255 },
         ...
         { "R": 255, "G": 255, "B": 191, "A": 255 },
      ],
      "interpolate": true
   },
   "mask": {
      "id": "Name of the band used as mask",
      "data_source": "/path/to/mask_data",
      "value": int,
      "bit_tests": [int]
   }
}
```

Description of each field:

* `name`: This is the name of the layer as exposed on the `<name>`
  in WMS GetCapabilities XML document.

* `title`: This is the title of the layer as exposed on the
  `<title>` in WMS GetCapabilities XML document.

* `abstract`: This is the abstract of the layer as exposed on the
  `<abstract>` in WMS GetCapabilities XML document.

* `data_source`: This field specifies the `/path/to/data` containing
  the files of the collection that needs to be exposed.

* `start_isodate`: For the datasets that contain a temporal
  dimension, this field specifies the date associated to the first
  layer. The date has to be specified using the UTC ISO date with
  seconds or milliseconds precision.

* `end_isodate`: For the datasets that contain a temporal dimension,
  this field specifies the date associated to the last layer. The date
  has to be specified using the UTC ISO date with seconds or
  milliseconds precision. For example, to find the date ranges for a
  dataset using MAS:

```
select min(po_min_stamp), max(po_max_stamp) from polygons where po_hash in (select pa_hash from paths where pa_parents @> array[md5('/g/data2/rs0/datacube/002/LS8_OLI_NBAR')::uuid]);
```

* `step_days`, `step_hours`, `step_minutes`: Indicates the number or
  days, hours or minutes between two consecutive dates.

* `accum`: This is a boolean value indicating MAS to search for
  either files with the exact date of the layer or files within the
  range `["start_isodate", "start_isodate"+"step_[days, hours,
  minutes]"]`. The value `true` is used in the case of exposing sparse
  datasets such as Landsat to create aggregated representations over a
  period of time.

* `time_generator`: This field specifies the method for generating
  the dates as exposed on the <time> field associated with the
  layer. The value `regular` adds the temporal step cumulatively to
  `start_isodate` until `end_isodate` is reached. If `time_generator`
  is set to `mas`, GSKY will attempt to pull timestamps from MAS. In
  this case, both `start_isodate` and `end_isodate` are optional.
  If they are set, GSKY will ask MAS to return all timestamps within
  the defined range. If `end_isodate` is not set, MAS will return
  timestamps before `now()`.

* `rgb_products`: List of the bands used to compose the image.
  These names map to the concept of `namespace`s on the MAS API.
  For WMS layers, currently GSKY implements the case of having either
  1 or 3 bands specified on this field. Details of band rendering
  please refer to the `Colour Palette` section. For WCS layers,
  `rgb_products` can have any number of bands.

* `offset_value`,`clip_value`,`scale_value`: These values are
  used to scale the dynamic range of the pixels in the collection to
  the `[0-255]` range used to render PNG or JPG images. This process
  and the relationship between the values is described in the next
  paragraph. Details please refer to the `Scaling of the pixel values`
  section. Note: These fields are not applicable to WCS.

* `legend_path`: Path to an image containing the legend for this
  layer. This file will be returned when a WMS GetLegend request is
  received for this layer.

* `zoom_limit`: This value specifies the maximum or highest zoom
  level that can be served. It uses meters/pixel -in the case of CRS
  expressed in meters-, to set this limitation.

* `palette`: Colour palette to render colour image for single-banded data
  Details please refer to the `Colour palette` section.

* `mask`: The band used to mask out the original data entries. Details
  please refer to the `Applying masks to data bands` section

### Colour palette

GSKY currently supports two modes of rendering tiles: RGB composites
and single band. If the `rgb_products` field contains three bands,
each of these bands is mapped into the red, green and blue channels of
the image respectively to generate a colour image (e.g. a PNG image).

If only one band is specified in the list an extra field needs to be
added to the JSON document to define the colour palette used to render
the image:

```
"palette": {
   "colours": [
      { "R": 215, "G": 25, "B": 28, "A": 255 },
      ...
      { "R": 255, "G": 255, "B": 191, "A": 255 },
  ],
  "interpolate": true
}
```

* `colours`: Contains a list of n (minimum of 3) RGB + Alpha colours
  used to define the colour ramp.

* `interpolate`: There are two different modes for defining the colour
  palettes. `"interpolate": true` defines an array of 256 colours
  evenly interpolating through the provided list of colours.
  `"interpolate": false` defines fixed colours within ranges of the
  [0-255] space using all the colours specified in the colours list.

### Scaling of the pixel values

For WMS layers, GSKY has options to scale pixel values before rendering
them into colour images. In order to scale values to within the [0-255]
range, three parameters can be defined: `offset_value`, `clip_value` and
`scale_value`. These three fields are applied to each pixel as in
the following formula:

`value = scale_value * (offset_value + min(pixel_value, clip_value))`

___Hint___: The `gdalinfo -mm` command provides the minimum and
maximum pixel values on a raster. This tool is useful to figure out
the appropriate values of the scale parameters when a new collection
needs to be exposed by GSKY.

### Applying masks to data bands

* `id`: Name of the band used as masks.

* `data_source` The path of the mask data files.

* `value` A 01 binary string used for computing the mask map using
  `and` operations. The mask map is computed as follows:
  The mask band and the original data bands are assumed to always have
  same array shape. The mask data band is bitwise `and`ed with
  the `value` field entry-wise. If the result is non-zero,
  the corresponding original data band entry is considered masked out.
  The `value` field alone is able to model logical conjunction
  cases.(i.e. `IF a AND b AND c THEN ...`). This field, however, will
  not be able to model a combination of logical conjunction and
  disjunction cases. (i.e.`IF a AND b OR c AND d THEN ...`). To model
  such cases, one will use the `bit_tests` field.

* `bit_tests`: An array of 01 binary strings. Each string in the
  `bit_tests` array will be `and` tested against the mask data band
  entry-wise. If any of the integer in the `bit_tests` array results in
  non-negative `and` test, the corresponding original data entry will be
  considered masked out. If both `value` and `bit_tests` fields are set,
  only the `value` field will be considered.

An example using the `bit_tests` field is as follows:

```
"mask": {
  "id": "pixelquality",
  "data_source": "/g/data2/rs0/datacube/002/LS8_OLI_PQ",
  "bit_tests": [
    "0000010000000000", "0000000000000000",
    "0000100000000000", "0000000000000000",
    "0001000000000000", "0000000000000000",
    "0010000000000000", "0000000000000000"
  ]
}
```

### Templated config files

Although it is possible to publish all the layers within a single `config.json`
file, it quickly becomes tedious because authoring the layers requires a lot of
manual cut-and-paste of `title`, `abstract`, `colour table` etc despite many
layers can share the common values of those. With templated config files, one
may organise the layers of a dataset like program code. An example to illustrate
the idea is as follows:

```
/<gsky config dir>
config.json
    /common
         abstract.txt
         colour_table.txt
    layer1.json
    layer2.json
    layer3.json
```

* There is a main `config.json` which `include` each layer file (e.g. `layer1.json`).
* Each layer file can also `include` the common artefacts such as `common/abstract.txt`
  for their corresponding data fields.

The underlying template engine GSKY uses is the Jet template engine. For the template
expression syntax, please refer to https://github.com/CloudyKit/jet/wiki/3.-Jet-template-syntax

### GSKY heredoc

Often times it is essential to be able to author multiline strings especially for
the `abstract` field of each layer. For example, one might want to include Markdown
text in order to have rich content. GSKY supports `heredoc` facility to allow
authoring multiline strings as shown below:

```
{
  "layers": [
     "name": "test layer"
     "abstract":
     $gdoc$

## Dataset tile
* dataset feature 1
* dataset feature 2

     $gdoc$
  ]
}
```

As can be seen from the above example, the text section enclosed by `$gdoc$` can
span multiple lines. Internally, GSKY automatically escapes the text section into
a valid single-line JSON string.
