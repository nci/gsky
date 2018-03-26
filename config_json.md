# GSKY configuration file description

GSKY can currently provide WMS and WPS services (as defined by the
Open Geosaptial Consortium or OGC). The GSKY configuration file is a
JSON document (with the filename `config.json`). It specifies the
datasets and configuration parameters that GSKY needs to expose the
different layers and services.

This document describes the structure and contents of the
configuration file.  New datasets can be added to this file so that
GSKY can expose new layers or services.


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

The document contains three keys at the top level:

* `service_config` -- Provides information about the fully-qualified
  domain name associated with the instance, MAS RESTful API endpoint
  and the list of worker nodes used to process the data.

* `layers` -- This field corresponds to the list of WMS layers
  exposed by GSKY. The structure of the documents defining the
  different layers is covered in the next section of this document.

* `processes` -- This field corresponds to the list of WPS services
  exposed by GSKY. This functionality is currently implemented for a
  specific case and there is not a generic way of defining new
  services. This part is not documented as the interface needs to be
  redefined to be more generic.

## WMS layers
A WMS layer is defined using a JSON document specifying values used
internally by GSKY to locate and process the data as well as
parameters used to describe the layer in the WMS GetCapabilities
response. There are different modes for exposing WMS layers which
require specific fields in the document, but the following document
contains the list of common parameters that need to be defined for any
layer.

GSKY currently supports two different modes of exposing WMS layers. 

### Single band WMS (Colour palette)
In the case where a single band needs to be exposed, a colour palette
needs to be defined in order to define the mapping between values and

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
   "time_generator": ["regular", "mcd43", "chirps20", "monthly", "yearly"],
   "rgb_products": [],
   "offset_value": float64,
   "clip_value": float64,
   "scale_value": float64,
   "legend_path": "path to image with legend",
   "zoom_limit": float64
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
  range ["start_isodate", "start_isodate"+"step_[days, hours,
  minutes]"]. The value `true` is used in the case of exposing sparse
  datasets such as Landsat to create aggregated representations over a
  period of time.
* `time_generator`: This field specifies the method for generating
  the dates as exposed on the <time> field associated with the
  layer. The value `regular` adds the temporal step cumulatively to
  `start_isodate` until `end_isodate` is reached.
* `rgb_products`: List of the bands used to compose the
  image. Currently GSKY implements the case of having either 1 or 3
  bands specified on this field. These names map to the concept of
  `namespace`s on the MAS API.
* `offset_value`,`clip_value`,`scale_value`: These values are
  used to scale the dynamic range of the pixels in the collection to
  the `[0-255]` range used to render PNG or JPG images. This process
  and the relationship between the values is described in the next
  paragraph.
* `legend_path`: Path to an image containing the legend for this
  layer. This file will be returned when a WMS GetLegend request is
  received for this layer.
* `zoom_limit`: This value specifies the maximum or highest zoom
  level that can be served. It uses meters/pixel -in the case of CRS
  expressed in meters-, to set this limitation.


Scaling of the pixel values in a collection:

In order to scale values to within the [0-255] range, three parameters
can be defined: `offset_value`, `clip_value` and
`scale_value`. These three fields are applied to each pixel as in
the following formula:

`value = scale_value * (offset_value + min(pixel_value, clip_value))`

___Hint___: The `gdalinfo -mm` command provides the minimum and
maximum pixel values on a raster. This tool is useful to figure out
the appropriate values of the scale parameters when a new collection
needs to be exposed by GSKY.

GSKY currently supports two modes of rendering tiles: RGB composites
and single band. If the `rgb_products` field contains three bands,
each of these bands is mapped into the red, green and blue channels of
the image respectively to generate a colour image.

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


