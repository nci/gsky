GSKY Docker Image
=================

The scripts under `gsky/docker` are meant for building GSKY docker images from
scratch. To build a docker image, please run the following command:

```
cd docker
docker build --build-arg sample_data_dir=geoglam_fc_sample_data/gdata \
    --build-arg gsky_repo="<unofficial gsky git repo/branch>" .
```

Build Arguments
---------------

1. `sample_data_dir` is the directory path for the sample data. To obtain sample
   data for reproducible builds, please pull the `v1` tagged image using the
   following command: `docker pull gjmouse/gsky:v1`. The sample data files are
   under `/gpath` of the `v1` image. Please also be aware that `sample_data_dir`
   must be a relative path under `gsky/docker` directory. This is a restriction
   imposed by docker `ADD` command.

2. `gsky_repo` is an optional build argument to specify any git repository/branch
   other than `http://github.com/nci/gsky.git`. For example, one might want to
   build an image for a feature branch using the following build arguments:
    `--build-arg gsky_repo="-b feature_branch http://github.com/<your own repo>`

Published Ports
---------------

The `Dockerfile` publishes port 8080 and 8888 for GSKY ows and MAS API services.
To access these ports from the host OS, one needs to use the `-p` option for
`docker run`. For example, `docker run --rm -it -p 8080:8080 <GSKY image>`

Sample Data
-----------

The sample data were taken from a subset of Geoglam Fractional Cover
http://www.geo-rapp.org/rapp-monitor/, which covers the entire Australia. The
sample data consist of 16 data files whose total size is about 311MB.

TerriaJS
--------

The `Dockerfile` also bundles TerriaJS https://github.com/TerriaJS/terriajs
for the purpose of visually demostrating GSKY WMS services on the sample data. 
To access TerriaJS from your web browser, please do the following:

1. Run `docker run --rm -it -p 8080:8080 <GSKY image>` to bring up a GSKY container.

2. TerriaJS can be accessed from `http://127.0.0.1:8080/terria` from your web browser.
