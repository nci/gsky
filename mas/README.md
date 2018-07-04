MAS Database
============
MAS stands for Metadata Storage. MAS uses Postgres JSON functions to extract the fields from the crawler outputs and generates materialized views for the RESTful API. This document intends to give instructions on deploying the AS Postgres database as well as ingesting crawler outputs into MAS. The `MAS_Design.md` provides technical details in database design.

Deploying MAS
-------------
Notes: The database superuser name must be postgres. This should not be problem if Postgres is installed with default settings. 
1. `export PGUSER=postgres`
2. `psql -f db/schema.sql`
3. `psql -d mas -f api/mas.sql`

Ingesting crawler outputs
-------------------------
The main script to ingest crawler outputs into MAS is `db/ingest_pipeline`.The usage of this script is as follows:
```
./ingest_pipeline.sh <shard> <crawl file1> ... <crawl fileN>
```
* `<shard>` is an identifier that uniquely identifies a shard. A shard can be regarded as logical collection of datasets under the same root data directory. For technical details about shards, please refer to `MAS_Design.md`
* `<crawl file1> ... <crawl fileN>` are the crawler outputs to get ingested.These crawl output files form logical collection of datasets under the same shard. 


