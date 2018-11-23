#!/bin/bash

here="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
shard=$1

(cd "$here" && psql -v ON_ERROR_STOP=1 -A -t -q -d mas <<EOD

truncate ${shard}.timestamps_cache;

set role mas;
set search_path to ${shard};

-- View of polygon metadata supplied by GDAL crawler, for GSKY
drop materialized view if exists polygons cascade;
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
      regexp_replace(trim(geo#>>'{namespace}'), '[^a-zA-Z0-9_]', '_', 'g')
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
;

create index poi_hash
  on polygons (po_hash);

create index poi_stamp
  on polygons (po_min_stamp, po_max_stamp);

create index poi_stamps
  on polygons using gin (po_stamps);

create index poi_name
  on polygons (po_name);

-- A list of SRID values for the schema, used for faster intersections
drop materialized view if exists polygon_srids cascade;
create materialized view polygon_srids as
  select
    distinct(public.st_srid(po_polygon))
      as ps_srid
    from
      polygons;

-- Test table. Probably to go...
drop view if exists geometries cascade;
create view geometries as
  select
    po_hash
      as go_hash,
    po_name
      as go_name,
    po_min_stamp
      as go_min_stamp,
    po_max_stamp
      as go_max_stamp,
    po_pixel_x
      as go_pixel_x,
    po_pixel_y
      as go_pixel_y,
    auth_name
      as go_auth_name,
    auth_srid
      as go_auth_srid,
    public.ST_AsText(po_polygon)
      as go_wkt,
    srtext
      as go_srtext,
    proj4text
      as go_proj4text
  from
    polygons,
    lateral (
      select
        *
      from
        public.spatial_ref_sys
      where
        srid = public.ST_SRID(po_polygon)
    ) a;

select refresh_views();
select refresh_polygons();

EOD
)

