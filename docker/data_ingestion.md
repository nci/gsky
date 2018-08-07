# Data Ingestion Tutorial

## Sample data

This guide provides walkthrough tutorial for user to ingest sample data and to publish a new layer for the sample data. The sample data we are going to use is the Geoglam fractional cover data located here:
`http://dapcm00.nci.org.au/thredds/catalog/tc43/modis-fc/v310/tiles/8-day/cover/catalog.html`
We choose to download the data files corresponding to the Australia region for year 2018. We also user GSKY docker image for demonstrating data ingestion steps.

## Step 1: Downloading sample data

Users can download the data files either into the container file system or to the host file system. For this tutorial, we demonstrate how to download files to the host file system and ingest them into the container later.
Before downloading them,  we first create a directory on the host file system that will store these files using the following command:
`demo@demo-pc:~$ mkdir docker_user_data && cd docker_user_data`

Then we use the following bash script to download the data files.

```bash
#!/bin/bash
url="http://dapcm00.nci.org.au/thredds/fileServer/tc43/modis-fc/v310/tiles/8-day/cover"
wget $url/FC.v310.MCD43A4.h27v12.2018.006.nc
wget $url/FC.v310.MCD43A4.h28v11.2018.006.nc
wget $url/FC.v310.MCD43A4.h28v12.2018.006.nc
wget $url/FC.v310.MCD43A4.h28v13.2018.006.nc
wget $url/FC.v310.MCD43A4.h29v10.2018.006.nc
wget $url/FC.v310.MCD43A4.h29v11.2018.006.nc
wget $url/FC.v310.MCD43A4.h29v12.2018.006.nc
wget $url/FC.v310.MCD43A4.h29v13.2018.006.nc
wget $url/FC.v310.MCD43A4.h30v10.2018.006.nc
wget $url/FC.v310.MCD43A4.h30v11.2018.006.nc
wget $url/FC.v310.MCD43A4.h30v12.2018.006.nc
wget $url/FC.v310.MCD43A4.h31v10.2018.006.nc
wget $url/FC.v310.MCD43A4.h31v11.2018.006.nc
wget $url/FC.v310.MCD43A4.h31v12.2018.006.nc
wget $url/FC.v310.MCD43A4.h32v10.2018.006.nc
```

## Step 2: Mapping `~/docker_user_data` from the host file system into container file system

This is a typical step of docker volume mapping. The data files will only be accessible from within the container after volume mapping. The following command maps host directory `/home/demo/docker_user_data` into container directory `/user_data`. Absolute path is required for volume mapping.

`demo@demo-pc:~$ docker run -rm --it -p 8080:8080 -v /home/demo/docker_user_data:/user_data gjmouse/gsky:v0`

## Step 3: Opening container shell

The rest of the steps of this tutorial need to be performed within the docker GSKY container. Thus we need to open a container shell in order to access the container. Firstly, we find out the ID of the GSKY container:

`demo@demo-pc:~$ docker ps`

```bash
CONTAINER ID        IMAGE               COMMAND                   ......
aa5d503869e6        gjmouse/gsky:v0     "/bin/sh -c ./gsky_e…"    ......
```

As can be seen, the GSKY container ID is `aa5d`. Next, we open the shell for this container using the following command:

`demo@demo-pc:~$ docker exec -it aa5d /bin/bash`
`root@aa5d503869e6:/#`

## Step 4: Crawling and ingesting the data files

GSKY indexing service requires metadata of the data files. Thus we need to crawl the data files to extract metadata followed by ingesting the metadata into the database. We use the following script to do so. As mentioned above, this step also needs to be performed from within the container.

```bash
#!/bin/bash
set -xeu

export PATH="/gsky/bin:/gsky/share/mas:$PATH"
export CRAWL_DIR=/user_data
export CRAWL_OUTPUT_DIR=/crawl_outputs
export CRAWL_CONC_LIMIT=2
export LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH:-}"

export PGUSER=postgres
export PGDATA=/pg_data

set +x
res=$(find "$CRAWL_DIR" -name "*.nc")
if [ -z "$res" ]
then
  echo "No *.nc files under '$CRAWL_DIR'."
  exit 1
fi
set -x

rm -rf $CRAWL_OUTPUT_DIR
mkdir -p $CRAWL_OUTPUT_DIR

/gsky/bin/gsky-crawl_pipeline.sh

crawl_job_id="${CRAWL_DIR//[\/]/_}"
(cd /gsky/share/mas && ./ingest_pipeline.sh $crawl_job_id /crawl_outputs/${crawl_job_id}_gdal.tsv.gz)
```

## Step 5: Publishing a new layer corresponding the sample data

The definition of the data layers is in GSKY config file. We open GSKY config file:

`root@aa5d503869e6:/# vi /gsky/etc/config.json`

The config file is a JSON file and the `layers` is an array of layer JSON object. We add the new layer JSON object into the `layers` array.

```json
   {
      "name":"geoglam:c6:frac_cover",
      "title":"geoglam fractional cover c6",
      "abstract":"fractional cover",
      "data_source":"/user_data",
      "time_generator":"mas",
      "rgb_products":[
        "bare_soil",
        "phot_veg",
        "nphot_veg"
      ],
      "clip_value":100,
      "zoom_limit":10000
    },
```

## Step 6: Reloading GSKY config file

The final step is to instruct GSKY to reload the config file we just edited. To do so, we send a `SIGUP` signal to GSKY `ows` process. We first find the `ows` process id:

`root@aa5d503869e6:/# ps a|grep ows`

```bash
35 pts/0    Sl+    0:07 ./gsky/bin/gsky-ows -p 8080
```

Thus the process id is 35. We then send the `SIGUP` signal to this process:

`root@aa5d503869e6:/# kill -1 35`

## Visualising the new layer in TerriaJS

To visualise the layer we just published, please go to `http://127.0.0.1:8080/terria` in your browser. Once the page is loaded, please click on `Add data` -> `My Data` -> `Add Web Data`. In the web data text box, please enter `http://127.0.0.1:8080/ows`. The new layer should appear in the layer list.
