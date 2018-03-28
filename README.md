GSKY: Distributed Scalable Geospatial Data Server
=================================================

What Is This?
-------------

GSK was developed at [NCI](http://nci.org.au) and is a scalable,
distributed server which presents a new approach for geospatial data
discovery and delivery using OGC standards.

[![Build Status](https://travis-ci.com/nci/gsky.svg?token=UnJphTz9GcxvBUhiHkKJ&branch=master)](https://travis-ci.com/nci/gsky)

License
-------

Copyright 2016, 2017, 2018 Australian National University

Licensed under the Apache License, Version 2.0 (the "License"); you
may not use this package except in compliance with the License.  A
copy of the [License](http://www.apache.org/licenses/LICENSE-2.0) may
be found in this source distribution in `LICENSE-2.0.txt`.

Configuration Files
-------------------

1. `config.json`: Contains the list of WMS and WPS services exposed by
   the server. It also contains the IP address of the index API used
   in the workflow.

2. `workers_config.json`: Contains the list of worker nodes specifying
   the IP address and list of ports per worker. Several workers can be
   specified on a single machine by adding several entries using the
   same IP address and different ports. These services have to be
   locally started at the specified machines.


How To Compile the Source
-------------------------

Dependencies:

+ Go > 1.6.0
+ GDAL > 2.1.0
+ Various Go packages listed below


Install required packages:

+ ```go get bitbucket.org/monkeyforecaster/geometry```
+ ```go get github.com/golang/protobuf/proto```
+ ```go get golang.org/x/net/context```
+ ```go get google.golang.org/grpc```
+ ```golang.org/x/crypto/ssh/terminal```

These packages can be done easily installed with `make get`.

Now compile the Go code with `configure` and then `make`. The
`configure` script takes all of the standard GNU `configure` flags
such as `--prefix` (to specify where to install GSKY). Once GSKY is
compiled, install it with `make install`.


How To Start the Server
-----------------------

- Start all the RPC worker nodes: `/opt/gsky/sbin/gsky-rpc -p 6000`

	The `-p` option sets the gRPC listening port. The default is port 6000.

- Start the main server: `/opt/gsky/sbin/gsky-ows -c 4`

	The `-c` option sets the level of concurrency at an RPC node.
