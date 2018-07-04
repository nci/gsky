# MAS Database

## Requirements

* PostgreSQL 9.6+
* Database "mas" (currently hardcoded)
* User "mas" (currently hardcoded) with full DDL/DML
* PostGIS 2.x in nci.public

## Design

* A sharded layout, where Postgres schemas == shards
* Shards are named to uniquely identify a logical collection of datasets. For example, the NCI GDATA project: `fr5`, `u39`, etc

Using shards allows individual projects to be relatively independent.

* Easy to truncate/reload within a shard without blocking others, or causing excessive vacuum load.
* Easy to split shards across servers using foreign tables.

## Public Relations

### public.paths

```
                  View "public.paths"
   Column    |            Type             | Modifiers
-------------+-----------------------------+-----------
 pa_hash     | uuid                        |
 pa_ingested | timestamp without time zone |
 pa_type     | path_type                   |
 pa_path     | text                        |
 pa_parents  | uuid[]                      |
```

A record:

```
               pa_hash                |        pa_ingested         |  pa_type  |   pa_path    |                                 pa_parents
--------------------------------------+----------------------------+-----------+--------------+-----------------------------------------------------------------------------
 3e1a4cf9-f0ec-945c-1697-5fcc76f556c0 | 2017-01-18 11:25:32.592855 | directory | /g/data3/lb4 | {399ab314-4e5e-e928-7ec4-94b96feb2d3f,8b33eb94-a838-49ce-1b58-d6f556f0a588}
```

The primary key `pa_hash` is created from the full filesystem path `pa_path`. All other `_hash` fields in the schema are foreign keys to a `paths` record.

```sql
nci=> select md5('/g/data3/lb4')::uuid as pa_hash;
               pa_hash
--------------------------------------
 3e1a4cf9-f0ec-945c-1697-5fcc76f556c0
(1 row)
```

For fast filesystem traversal, the `pa_parents` array stores the `pa_hash` of each directory level. For `/g/data3/lb4`:

```
{399ab314-4e5e-e928-7ec4-94b96feb2d3f,8b33eb94-a838-49ce-1b58-d6f556f0a588}
 |--------------- /g ---------------| |------------ /g/data3 ------------|
```

Storing paths like this means:

* Flexible traversal using Postgres array functions and a GIN index
* Comparable disk usage as storing raw text paths with indexes (when the necessary denormalization with that approach is taken into account)
* Better performance than the proper Nth-normal-form hierachical schema, which must be traversed via a slow recursive CTE or other very creative SQL

Notice that `public.paths` is an SQL view; it's basically a `UNION` of all shards (schemas), each of which have their own `paths` table with the same structure. The other public views work the same way. This means cross-shard GDATA-wide queries are possible for reporting purposes, but generally it's better to go straight to the shard of interest.

### public.files

```
                            View "public.files"
  Column   |           Type           | Modifiers | Storage  | Description
-----------+--------------------------+-----------+----------+-------------
 fi_hash   | uuid                     |           | plain    |
 fi_parent | uuid                     |           | plain    |
 fi_name   | text                     |           | extended |
 fi_size   | bigint                   |           | plain    |
 fi_ctime  | timestamp with time zone |           | plain    |
 fi_mtime  | timestamp with time zone |           | plain    |
 fi_atime  | timestamp with time zone |           | plain    |
 fi_mode   | text                     |           | extended |
 fi_inode  | text                     |           | extended |
 fi_uid    | text                     |           | extended |
 fi_gid    | text                     |           | extended |
 fi_user   | text                     |           | extended |
 fi_group  | text                     |           | extended |
```

### public.directories

```
                         View "public.directories"
  Column   |           Type           | Modifiers | Storage  | Description
-----------+--------------------------+-----------+----------+-------------
 di_hash   | uuid                     |           | plain    |
 di_parent | uuid                     |           | plain    |
 di_name   | text                     |           | extended |
 di_ctime  | timestamp with time zone |           | plain    |
 di_mtime  | timestamp with time zone |           | plain    |
 di_atime  | timestamp with time zone |           | plain    |
 di_mode   | text                     |           | extended |
 di_inode  | text                     |           | extended |
 di_uid    | text                     |           | extended |
 di_gid    | text                     |           | extended |
 di_user   | text                     |           | extended |
 di_group  | text                     |           | extended |
 di_count  | numeric                  |           | main     |
 di_size   | numeric                  |           | main     |
```

### public.links

```
                            View "public.links"
  Column   |           Type           | Modifiers | Storage  | Description
-----------+--------------------------+-----------+----------+-------------
 li_hash   | uuid                     |           | plain    |
 li_parent | uuid                     |           | plain    |
 li_name   | text                     |           | extended |
 li_ctime  | timestamp with time zone |           | plain    |
 li_mtime  | timestamp with time zone |           | plain    |
 li_atime  | timestamp with time zone |           | plain    |
 li_inode  | text                     |           | extended |
 li_uid    | text                     |           | extended |
 li_gid    | text                     |           | extended |
 li_user   | text                     |           | extended |
 li_group  | text                     |           | extended |
 li_intact | boolean                  |           | plain    |
 li_target | text                     |           | extended |
```

## Shard Relations

Each shard has a local copy of `paths`, `files`, `directories` and `links` as real tables or materialized views, with indexes.

```
                                        Unlogged table "lb4.paths"
   Column    |            Type             |       Modifiers        | Storage  | Stats target | Description
-------------+-----------------------------+------------------------+----------+--------------+-------------
 pa_hash     | uuid                        | not null               | plain    |              |
 pa_ingested | timestamp without time zone | not null default now() | plain    |              |
 pa_type     | path_type                   |                        | plain    |              |
 pa_path     | text                        | not null               | extended |              |
 pa_parents  | uuid[]                      |                        | extended |              |
Indexes:
    "paths_pkey" PRIMARY KEY, btree (pa_hash)
    "pai_parent" btree ((pa_parents[array_length(pa_parents, 1)]))
    "pai_parents" gin (pa_parents)
    "pai_type" btree (pa_type)
```

...etc

### metadata

The shard `metadata` table stores free-form JSON metadata records (schemaless documents) mapped to paths.

```
                                       Unlogged table "lb4.metadata"
   Column    |            Type             |       Modifiers        | Storage  | Stats target | Description
-------------+-----------------------------+------------------------+----------+--------------+-------------
 md_hash     | uuid                        | not null               | plain    |              |
 md_ingested | timestamp without time zone | not null default now() | plain    |              |
 md_type     | text                        | not null               | extended |              |
 md_json     | jsonb                       | not null               | extended |              |
Indexes:
    "mdi_pk" UNIQUE, btree (md_hash, md_type)
```

## GSKY metadata
The GSKY metadata JSON is used to generate a `polygons` materialized view across all files, with indexes for PostGIS and general access via the API (notice the JSON field extraction within the sub-query).

```sql
nci=# \d+ polygons
                               Materialized view "u39.polygons"
    Column    |            Type            | Modifiers | Storage  | Stats target | Description
--------------+----------------------------+-----------+----------+--------------+-------------
 po_hash      | uuid                       |           | plain    |              |
 po_stamps    | timestamp with time zone[] |           | extended |              |
 po_min_stamp | timestamp with time zone   |           | plain    |              |
 po_max_stamp | timestamp with time zone   |           | plain    |              |
 po_name      | text                       |           | extended |              |
 po_pixel_x   | numeric                    |           | main     |              |
 po_pixel_y   | numeric                    |           | main     |              |
 po_polygon   | public.geometry            |           | main     |              |
Indexes:
    "poi_hash" btree (po_hash)
    "poi_name" btree (po_name)
    "poi_polygon_100000" gist (po_polygon) WHERE public.st_srid(po_polygon) = 100000
    "poi_polygon_100001" gist (po_polygon) WHERE public.st_srid(po_polygon) = 100001
    "poi_stamp" btree (po_min_stamp, po_max_stamp)
    "poi_stamps" gin (po_stamps)
View definition:
create materialized view polygons as
  select
    hash
      as po_hash,
    array(select jsonb_array_elements_text(stamps))::timestamptz[]
      as po_stamps,
    (select min(t) from unnest(array(select jsonb_array_elements_text(stamps))::timestamptz[]) t)
      as po_min_stamp,
    (select max(t) from unnest(array(select jsonb_array_elements_text(stamps))::timestamptz[]) t)
      as po_max_stamp,
    variable
      as po_name,
    (geotransform->>1)::numeric
      as po_pixel_x,
    (geotransform->>5)::numeric
      as po_pixel_y,
    public.st_geomfromtext(polygon, srid)
      as po_polygon
  from (
    select
      hash,
      trim(geo#>>'{polygon}')
        as polygon,
      trim(geo#>>'{proj_wkt}')
        as srtext,
      trim(geo#>>'{proj4}')
        as proj4text,
      trim(geo#>>'{namespace}')
        as variable,
      geo#>'{timestamps}'
        as stamps,
      geo#>'{geotransform}'
        as geotransform
    from (
      select
        md_hash
          as hash,
        jsonb_array_elements(md_json->'geo_metadata')
          as geo
      from metadata
      where
      md_type = 'gdal'
      and jsonb_typeof(md_json->'geo_metadata') = 'array'
    ) a
  ) b
  join public.spatial_ref_sys s
    on b.srtext = s.srtext
    and b.proj4text = s.proj4text
```

## Other possible metadata records
For example, it is also possible to have POSIX metadata records:

```sql
nci=> select jsonb_pretty(md_json) from metadata where md_hash = md5('/g/data3/lb4')::uuid and md_type = 'posix';
           jsonb_pretty
----------------------------------
 {
     "gid": 5641,
     "uid": 0,
     "mode": "drwxrwx--x",
     "size": 4096,
     "type": "directory",
     "user": "root",
     "atime": 1480296416,
     "ctime": 1465434771,
     "group": "lb4",
     "inode": 144115497095480000,
     "mtime": 1465434771
 }
(1 row)
```

Arbitrary `md_type` values are fine. So far there has been: `posix`, `magic` (libmagic crawler, mime types and stuff), `checksum` (md5 crawler) and `gdal` (Pablo's crawler for NetCDF/HDF headers). An Amazon S3 bucket crawler is in the works, which will lack `posix` records in favour of... something else.

Shard materialized views such as `files` extract relevant `metadata` records using Postgres JSON functions, apply proper type casting, add indexes, etc. They are regenerated periodically to take newly ingested crawls or streamed filesystem changes into account.

```sql
create materialized view files as
  select
    paths.pa_hash
      as fi_hash,
    paths.pa_parents[array_upper(paths.pa_parents, 1)]
      as fi_parent,
    split_part(paths.pa_path, '/', length(paths.pa_path) - length(replace(paths.pa_path, '/', '')) + 1)
      as fi_name,
    (metadata.md_json->>'size')::bigint
      as fi_size,
    to_timestamp(((metadata.md_json->>'ctime')::bigint)::double precision)
      as fi_ctime,
    to_timestamp(((metadata.md_json->>'mtime')::bigint)::double precision)
      as fi_mtime,
    to_timestamp(((metadata.md_json->>'atime')::bigint)::double precision)
      as fi_atime,
    metadata.md_json->>'mode'
      as fi_mode,
    metadata.md_json->>'inode'
      as fi_inode,
    metadata.md_json->>'uid'
      as fi_uid,
    metadata.md_json->>'gid'
      as fi_gid,
    metadata.md_json->>'user'
      as fi_user,
    metadata.md_json->>'group'
      as fi_group
  from paths
    left join metadata
      on metadata.md_hash = paths.pa_hash
      and metadata.md_type = 'posix'
  where
    paths.pa_type = 'file'::path_type
;
```


