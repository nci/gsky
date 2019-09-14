GSKY Metrics Logging Format
=================================================

GSKY metrics logging facility captures a wide range of statistics
during the lifecycle of a WMS/WCS/WPS request. The lifecyle of the
request will mainly involve three server components - OWS, MAS and
workers. The json-based logging format is therefore structured to
reflect these server components. A high-level overview of the json
formatted structure is as follows:

```json
{
  <ows metrics>

  "indexer": {
    <MAS metrics>
  },

  "rpc": {
    <worker metrics>
  }
}
```

A concrete example to show what metrics have been logged is as follows:

```json
{
  "req_time": "2019-08-27T11:57:19.211Z",
  "req_duration": 515357769,
  "url": {
    "raw_url": "/ows/geoglam?time=2018-12-27T00:00:00.000Z&srs=EPSG:3857&transparent=true&format=image/png&exceptions=application/vnd.ogc.se_xml&styles=&tiled=true&feature_count=101&service=WMS&version=1.1.1&request=GetMap&layers=global:c6:frac_cover&bbox=15028131.257091936,-7514065.628545966,17532819.79994059,-5009377.085697312&width=256&height=256",
    "host": "",
    "path": "/ows/geoglam",
    "query": {
      "bbox": "15028131.257091936,-7514065.628545966,17532819.79994059,-5009377.085697312",
      "exceptions": "application/vnd.ogc.se_xml",
      "feature_count": "101",
      "format": "image/png",
      "height": "256",
      "layers": "global:c6:frac_cover",
      "request": "GetMap",
      "service": "WMS",
      "srs": "EPSG:3857",
      "styles": "",
      "tiled": "true",
      "time": "2018-12-27T00:00:00.000Z",
      "transparent": "true",
      "version": "1.1.1",
      "width": "256"
    }
  },
  "remote_addr": "192.168.1.100",
  "remote_host": "192.168.1.100",
  "remote_port": "",
  "http_status": 200,
  "indexer": {
    "duration": 29391537,
    "url": {
      "raw_url": "http://10.0.0.100:8888/data_path/tiles/8-day/cover?intersects&metadata=gdal&time=2018-12-27T00:00:00.000Z&until=2019-01-04T00:00:00.000Z&srs=EPSG:3857&wkt=POLYGON%20((15028131.257092%20-7514065.628546,%2017532819.799941%20-7514065.628546,%2017532819.799941%20-5009377.085697,%2015028131.257092%20-5009377.085697,%2015028131.257092%20-7514065.628546))&namespace=bare_soil,phot_veg,nphot_veg&nseg=2&limit=-1",
      "host": "10.0.0.100:8888",
      "path": "/data_path/tiles/8-day/cover",
      "query": {
        "intersects": "",
        "limit": "-1",
        "metadata": "gdal",
        "namespace": "bare_soil,phot_veg,nphot_veg",
        "nseg": "2",
        "srs": "EPSG:3857",
        "time": "2018-12-27T00:00:00.000Z",
        "until": "2019-01-04T00:00:00.000Z",
        "wkt": "POLYGON ((15028131.257092 -7514065.628546, 17532819.799941 -7514065.628546, 17532819.799941 -5009377.085697, 15028131.257092 -5009377.085697, 15028131.257092 -7514065.628546))"
      }
    },
    "geometry": "POLYGON ((-55.7765730186679 135.0,-55.7765730186679 157.5,-40.979898069618 157.5,-40.979898069618 135.0,-55.7765730186679 135.0))",
    "geometry_area": 332.9251863536672,
    "num_files": 24,
    "num_granules": 24
  },
  "rpc": {
    "duration": 495429784,
    "num_tiled_granules": 24,
    "bytes_read": 50112000,
    "user_time": 646840680000,
    "sys_time": 46162316000
  }
}
```

The metrics for each of the three server components will be discussed
as follows:

OWS metrics
-------------------------------------------------

* `req_time`: The datetime the request is made.

* `req_duration`: Number of nanoseconds taken to process the request.
  This quantity is the total processing time of the request including
  OWS, MAS, workers and any other overheads.

* `url`: Request URL. The URL fields are extracted into json fields for
  the ease of search.

* `remote_addr`: Client IP address and TCP port. If OWS is behind reverse
  proxy, OWS will first look for `X-Forwarded-For` HTTP header. Failing that,
  OWS will look for `X-Real-IP` HTTP header.

* `remote_host`: The IP part of `remote_addr`.

* `remote_port`: The TCP port part of `remote_addr`.

* `http_status`: HTTP status code.

MAS/Indexer Metrics
-------------------------------------------------

* `duration`: Number of nanoseconds taken to process the indexer query.

* `url`: Indexer URL. The URL fields are extracted into json fields for
  the ease of search.

* `geometry`: Bounding box geometry in EPSG:4326 in WKT format.

* `geometry_area`: Area of the bounding box geometry in EPSG:4326.

* `num_files`: Number of data files returned from the MAS.

* `num_granules`: Number of granules filtered from the data files. More
  often than not, `num_granules` is equal to `num_files`. But for a
  variety of reasons such as multiple bands, sparse bands along the time
  axis, etc, those two quantities are no longer equal. In general,
  `num_granules` should be considered the effective workload sent to the
  backend.

Worker/RPC Metrics
-------------------------------------------------

* `duration`: Number of nanoseconds taken to process the RPC requests.
  This quantity is the response wall time of the backend workers.

* `num_tiled_granules`: The actual number of granules sent to the
  workers. This quantity is equivalent to number of RPC calls to the
  workers. More often than not, this quantity is equal to `num_granules`.
  But if OWS is configured to dynamically tile the request on-the-fly
  to improve parallelism, `num_tiled_granules` may be greater than
  `num_granules`.Dynamic request tiling, however, does not change the
  overall workload for the workers.

* `bytes_read`: Total number of bytes read from the data files associated
  with the current request.

* `user_time`: Total number of CPU time in userspace in nanoseconds.
  This quantity is different from `duration`. The former is CPU time
  accumulated from all workers while the later is worker response wall
  time.

* `sys_time`: Total number of CPU time in kernel space in nanoseconds.
