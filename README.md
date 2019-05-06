GSKY: Distributed Scalable Geospatial Data Server
=================================================

What Is This?
-------------

GSKY was developed at [NCI](http://nci.org.au) and is a scalable,
distributed server which presents a new approach for geospatial data
discovery and delivery using OGC standards. The most recent release is
[here](https://github.com/nci/gsky/releases).

License
-------

Copyright 2016 - 2019 Australian National University

Licensed under the Apache License, Version 2.0 (the "License"); you
may not use this package except in compliance with the License.  A
copy of the [License](http://www.apache.org/licenses/LICENSE-2.0) may
be found in this source distribution in `LICENSE-2.0.txt`.

Contributions
-------------

Suggestions, enhancement requests, bug reports and patches to GSKY are
welcome via this GitHub page. Please submit patches as a GitHub pull
request. Authors retain copyright over their contributions.

[![Build Status](https://travis-ci.org/nci/gsky.svg?branch=master)](https://travis-ci.org/nci/gsky)

Citing GSKY in publications
---------------------------

When referring to GSKY in publications please use the citation in
[CITATION.md](CITATION.md).  A ready-to-use BibTeX entry for LaTeX
users can also be found in this file.

Configuration Files
-------------------

1. `config.json`: Contains the list of WMS, WCS and WPS layers exposed by
   the server. It also contains the IP address of the index API used
   in the workflow. In addtion, it contains the list of worker nodes 
   specifying the IP address and list of ports per worker. Several workers 
   can be specified on a single machine by adding several entries using 
   the same IP address and different ports. These services have to be
   locally started at the specified machines.

2. Serveral `config.json` files can be organized into directories to form
   namespaces to group logical collection of datasets together.
   For example, the server serves two science projects with the following
   URLs:

   ```
   1) http://<server address>/ows/project1
   2) http://<server address>/ows/project2
   ```

   The directory structure of the config files will be as follows:

   ```
   <config root directory>

       project1
          config.json

       project2
          config.json
   ```

How To Compile the Source
-------------------------

Dependencies:

+ Go > 1.6.0
+ GDAL > 2.1.0
+ Various Go packages (handled by the build system)

```console
$ export GOPATH=~/go
$ go get github.com/nci/gsky
$ cd $GOPATH/src/github.com/nci/gsky
$ ./configure
$ make all install
```

The `configure` script takes all of the standard GNU `configure` flags
such as `--prefix` (to specify where to install GSKY).

Overview of the Servers
-----------------------

GSKY mainly consists of three servers working together to deliver services. The main server (`ows.go`) is the front-end server that takes WMS/WCS/WPS HTTP requests as inputs. The main server talks to the MAS Restful API server (`mas/api/api.go`) for the data files that intersect with the polygon bounding box in the WMS/WCS/WPS requests. With those data files, the main server talks to the RPC worker nodes (`grpc-server/main.go`) for compute and IO intensive tasks and then sends the results back to the client side.

How To Start the Servers
-----------------------

- Start the MAS Restful API server: `/opt/gsky/sbin/masapi -port 8888`

	The `-port` option sets the API server listening port. The default is port 8080.

- Start all the RPC worker nodes: `/opt/gsky/sbin/gsky-rpc -p 6000`

	The `-p` option sets the gRPC listening port. The default is port 6000.

- Start the main server: `/opt/gsky/sbin/gsky-ows -p 8080`

	The `-p` option sets the main server listening port. The default is port 8080.
