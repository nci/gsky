-- Metadata

-- Copyright (c) 2016, NCI, Australian National University.

-- Create a shard schema.
-- Mostly, shard == NCI project.

-- Crawlers are expected to output a 3-column tab-delimited file:
--
-- <gdata_path>\t<metadata_type>\t<json_blob>
--
-- Ingestion is just a matter of piping it line-by-line to the
-- following "ingest" table. An INSERT trigger handles parsing. Eg:
--
-- cat <tsv> | psql -c "set search_path to ${shard}; copy ingest from stdin with (format 'csv', delimiter E'\t', quote E'\b');"
--
-- Or with GNU Parallel:
--
-- cat <tsv> | parallel -j 8 --pipe -L 100000 --retries 10 --will-cite \
--   'psql -c "set search_path to ${shard}; copy ingest from stdin with (format 'csv', delimiter E'\t', quote E'\b');"'

set role mas;

-- Data ingest holding yard
drop table if exists ingest cascade;
create unlogged table ingest (
  in_path text,
  in_type text,
  in_json jsonb
);

-- Ingest a record from a crawler (3-column TSV)
drop function if exists ingest_line();
create function ingest_line()
  returns trigger language plpgsql as $$

  begin

    set client_min_messages to error;

    -- This is probably part of a bulk insert operation, probably even parallel ops.
    -- In order to reduce lock contention, buffer records in session temporary tables
    -- until we have the whole batch.
    create temporary table if not exists mypaths (
      pa_hash uuid not null primary key,
      pa_ingested timestamptz not null default now(),
      pa_type public.path_type default null,
      pa_path text not null,
      pa_parents uuid[]
    );

    create temporary table if not exists mymetadata (
      md_hash uuid not null,
      md_ingested timestamptz not null default now(),
      md_type text not null,
      md_json jsonb not null
    );

    set client_min_messages to warning;

    insert into mymetadata (
      md_hash,
      md_type,
      md_json
    )
    values (
      md5(trim(new.in_path))::uuid,
      trim(new.in_type),
      new.in_json
    );

    case

      when trim(new.in_type) = 'posix' then

        insert into mypaths (
          pa_hash,
          pa_type,
          pa_path
        )
        values (
          md5(trim(new.in_path))::uuid,
          (new.in_json->>'type')::public.path_type,
          trim(new.in_path)
        )
        on conflict (pa_hash)
        do update set
          pa_type = excluded.pa_type,
          pa_ingested = now()
        ;

      else

        insert into mypaths (
          pa_hash,
          pa_type,
          pa_path
        )
        values (
          md5(trim(new.in_path))::uuid,
          null,
          trim(new.in_path)
        )
        on conflict (pa_hash)
        do update set
          pa_ingested = now()
        ;

    end case;

    return null;
  end
$$;

drop trigger if exists ingest on ingest cascade;
create trigger ingest before insert on ingest
  for each row execute procedure ingest_line();

-- Ingest a batch of crawler records (3-column TSV)
drop function if exists ingested_lines();
create function ingested_lines()
  returns trigger language plpgsql as $$

  begin

    update mypaths set pa_parents = (
      select
        array_agg(md5(t)::uuid order by octet_length(t))
      from
        public.parent_paths(pa_path) t
    );

    insert into paths
      select * from mypaths
    on conflict (pa_hash)
      do update set
        pa_ingested = excluded.pa_ingested
    ;

    drop table mypaths;

    insert into metadata select * from mymetadata
      on conflict (md_hash, md_type)
      do update set
        md_ingested = excluded.md_ingested,
        md_json = excluded.md_json
    ;

    drop table mymetadata;

    return null;
  end
$$;

create trigger ingested after insert on ingest
  for each statement execute procedure ingested_lines();

-- All filesystem objects: directories, files, links
drop table if exists paths cascade;
create table paths (
  pa_hash uuid not null primary key,
  pa_ingested timestamptz not null default now(),
  pa_type public.path_type default null,
  pa_path text not null,
  pa_parents uuid[]
);

create index pai_type
  on paths (pa_type);

create index pai_parents
  on paths using gin (pa_parents);

create index pai_parent
  on paths ((pa_parents[array_length(pa_parents, 1)]));

-- Using a proper recursive CTE for aggregating directory counts/sizes for directories
-- is slow. Instead, maintain a table of counters for doing the it iteratively
drop table if exists tallies cascade;
create unlogged table tallies (
  ta_hash uuid not null primary key,
  ta_count bigint not null,
  ta_size bigint not null
);

drop table if exists metadata cascade;

-- Raw crawler JSON blobs
create table metadata (
  md_hash uuid not null,
  md_ingested timestamptz not null default now(),
  md_type text not null,
  md_json jsonb not null
);

create unique index mdi_pk
  on metadata (md_hash, md_type);

-- View of paths + metadata for files
drop materialized view if exists files cascade;
create materialized view files as
  select
    pa_hash
      as fi_hash,
    pa_parents[array_upper(pa_parents, 1)]
      as fi_parent,
    split_part(pa_path, '/', length(pa_path) - length(replace(pa_path, '/', '')) + 1)
      as fi_name,
    (md_json->>'size')::bigint
      as fi_size,
    to_timestamp((md_json->>'ctime')::bigint)
      as fi_ctime,
    to_timestamp((md_json->>'mtime')::bigint)
      as fi_mtime,
    to_timestamp((md_json->>'atime')::bigint)
      as fi_atime,
    md_json->>'mode'
      as fi_mode,
    md_json->>'inode'
      as fi_inode,
    md_json->>'uid'
      as fi_uid,
    md_json->>'gid'
      as fi_gid,
    md_json->>'user'
      as fi_user,
    md_json->>'group'
      as fi_group
  from
    paths
  left join
    metadata
      on md_hash = pa_hash and md_type = 'posix'
  where
    pa_type = 'file'
;

create unique index fii_hash
  on files (fi_hash);

--create index fii_parent
--  on files (fi_parent);

-- View of paths + metadata for links
drop materialized view if exists links cascade;
create materialized view links as
  select
    pa_hash
      as li_hash,
    pa_parents[array_upper(pa_parents, 1)]
      as li_parent,
    split_part(pa_path, '/', length(pa_path) - length(replace(pa_path, '/', '')) + 1)
      as li_name,
    to_timestamp((md_json->>'ctime')::bigint)
      as li_ctime,
    to_timestamp((md_json->>'mtime')::bigint)
      as li_mtime,
    to_timestamp((md_json->>'atime')::bigint)
      as li_atime,
    md_json->>'inode'
      as li_inode,
    md_json->>'uid'
      as li_uid,
    md_json->>'gid'
      as li_gid,
    md_json->>'user'
      as li_user,
    md_json->>'group'
      as li_group,
    md_json->>'intact'
      as li_intact,
    md_json->>'target'
      as li_target
  from
    paths
  left join
    metadata
      on md_hash = pa_hash and md_type = 'posix'
  where
    pa_type = 'link'
;

create unique index lii_hash
  on links (li_hash);

--create index lii_parent
--  on links (li_parent);

-- View of paths + metadata + tallies for directories
drop materialized view if exists directories cascade;
create materialized view directories as
  select
    pa_hash
      as di_hash,
    pa_parents[array_upper(pa_parents, 1)]
      as di_parent,
    split_part(pa_path, '/', length(pa_path) - length(replace(pa_path, '/', '')) + 1)
      as di_name,
    to_timestamp((md_json->>'ctime')::bigint)
      as di_ctime,
    to_timestamp((md_json->>'mtime')::bigint)
      as di_mtime,
    to_timestamp((md_json->>'atime')::bigint)
      as di_atime,
    md_json->>'mode'
      as di_mode,
    md_json->>'inode'
      as di_inode,
    md_json->>'uid'
      as di_uid,
    md_json->>'gid'
      as di_gid,
    md_json->>'user'
      as di_user,
    md_json->>'group'
      as di_group,
    coalesce(ta_count, 0)
      as di_count,
    coalesce(ta_size, 0)
      as di_size
  from
    paths
  left join
    tallies
      on pa_hash = ta_hash
  left join
    metadata
      on md_hash = pa_hash and md_type = 'posix'
  where
    pa_type = 'directory'
;

create unique index dii_hash
  on directories (di_hash);

-- Extracted NetCDF headers
drop view if exists netcdf cascade;
create view netcdf as
  select
    fi_hash
      as ne_hash,
    (md_json->>'format')
      as ne_format,
    (md_json->'attributes'->>'id')
      as ne_id,
    (md_json->'attributes'->>'title')
      as ne_title,
    (md_json->'attributes'->>'summary')
      as ne_summary,
    (md_json->'attributes'->>'source')
      as ne_source,
    (md_json->'attributes'->>'keywords')
      as ne_keywords,
    public.try_timestamptz(md_json->'attributes'->>'date_created')
      as ne_date_created,
    public.try_timestamptz(md_json->'attributes'->>'date_modified')
      as ne_date_modified,
    public.try_timestamptz(md_json->'attributes'->>'date_issued')
      as ne_date_issued,
    (md_json->'attributes'->>'Conventions')
      as ne_conventions,
    (md_json->'attributes'->>'history')
      as ne_history,
    (md_json->'attributes'->>'metadata_link')
      as ne_metadata_link,
    (md_json->'attributes'->>'license')
      as ne_license,
    (md_json->'attributes'->>'doi')
      as ne_doi,
    (md_json->'attributes'->>'product_version')
      as ne_product_version,
    (md_json->'attributes'->>'processing_level')
      as ne_processing_level,
    (md_json->'attributes'->>'institution')
      as ne_institution,
    (md_json->'attributes'->>'project')
      as ne_project,
    (md_json->'attributes'->>'instrument')
      as ne_instrument,
    (md_json->'attributes'->>'platform')
      as ne_platform,
    (md_json->'attributes'->>'references')
      as ne_references,
    (md_json->'attributes'->>'standard_name_vocabulary')
      as ne_standard_name_vocabulary,
    (md_json->'attributes'->>'geospatial_lat_min')::double precision
      as ne_geospatial_lat_min,
    (md_json->'attributes'->>'geospatial_lat_max')::double precision
      as ne_geospatial_lat_max,
    (md_json->'attributes'->>'geospatial_lon_min')::double precision
      as ne_geospatial_lon_min,
    (md_json->'attributes'->>'geospatial_lon_max')::double precision
      as ne_geospatial_lon_max,
    (md_json->'attributes'->>'geospatial_vertical_min')::double precision
      as ne_geospatial_vertical_min,
    (md_json->'attributes'->>'geospatial_vertical_max')::double precision
      as ne_geospatial_vertical_max,
    (md_json->'attributes'->>'geospatial_vertical_positive')
      as ne_geospatial_vertical_positive,
    (md_json->'attributes'->>'geospatial_bounds')
      as ne_geospatial_bounds,
    public.try_timestamptz(md_json->'attributes'->>'time_coverage_start')
      as ne_time_coverage_start,
    public.try_timestamptz(md_json->'attributes'->>'time_coverage_end')
      as ne_time_coverage_end,
    (md_json->'attributes'->>'time_coverage_duration')
      as ne_time_coverage_duration,
    (md_json->'attributes'->>'time_coverage_resolution')
      as ne_time_coverage_resolution,
    md_json
      as ne_json
  from
    files
  inner join
    metadata
      on fi_hash = md_hash
      and md_type = 'netcdf'
;

-- Refresh the state of this schema after an ingest operation
drop function if exists refresh_views();
create function refresh_views()
  returns boolean language plpgsql as $$

  declare
    levels bigint;
    rec record;
  begin

    raise notice 'refresh files';
    refresh materialized view files;

    raise notice 'refresh links';
    refresh materialized view links;

    levels := coalesce((select max(array_length(pa_parents, 1)) from paths), 0);
    raise notice 'max path depth %', levels;

    truncate tallies;

    for level in reverse levels .. 0 loop
      raise notice 'tallies depth %', level;

      insert into tallies
        select
          p.pa_hash,
          count(c.pa_hash) + sum(coalesce(ta_count, 0)),
          sum(coalesce(fi_size, 0)) + sum(coalesce(ta_size, 0)) + 4096
        from
          paths p
        left join
          paths c
            on c.pa_parents[array_length(c.pa_parents, 1)] = p.pa_hash
        left join
          tallies t
            on c.pa_hash = ta_hash
        left join
          files f
            on c.pa_hash = fi_hash
        where
          array_length(p.pa_parents, 1) = level
          and p.pa_type = 'directory'
        group by
          p.pa_hash
      on conflict (ta_hash)
        do update set
          ta_count = excluded.ta_count,
          ta_size = excluded.ta_size
      ;
    end loop;

    raise notice 'refresh directories';
    refresh materialized view directories;

    return true;
  end
$$;

create or replace function refresh_polygons()
  returns boolean language plpgsql as $$

  declare

    rec record;
    pg_version int;

  begin

    insert into public.nci_spatial_ref_sys (auth_name, srtext, proj4text)
      select
        'NCI',
        b.srtext,
        b.proj4text
      from (
        select
          trim(geo#>>'{proj_wkt}')
            as srtext,
          trim(geo#>>'{proj4}')
            as proj4text
        from (
          select
            jsonb_array_elements(md_json->'geo_metadata')
              as geo
          from
            metadata
          where
            md_type = 'gdal'
            and jsonb_typeof(md_json->'geo_metadata') = 'array'
        ) a
        group by srtext, proj4text
      ) b
      left join public.spatial_ref_sys s1
        on b.srtext = s1.srtext
        and b.proj4text = s1.proj4text
      left join public.nci_spatial_ref_sys s2
        on b.srtext = s2.srtext
        and b.proj4text = s2.proj4text
      where
        s1.srtext is null
        and s2.srtext is null
    ;

    insert into public.spatial_ref_sys (srid, auth_name, auth_srid, srtext, proj4text)
      select srid, auth_name, srid, srtext, proj4text from public.nci_spatial_ref_sys
    on conflict (srid) do nothing;

    pg_version := (select split_part(setting, '.', 1)::int
      from pg_settings where name = 'server_version');

    if pg_version >= 11 then
      raise notice 'PG version %, enabling parallel query', pg_version;
      set parallel_setup_cost = 1;
      set parallel_tuple_cost = 0.000001;
      set max_parallel_workers = 4;
      set max_parallel_workers_per_gather = 4;
      set max_parallel_maintenance_workers = 4;
      set synchronous_commit = 'off';
      set work_mem = '1GB';
      set maintenance_work_mem = '1GB';
      set effective_cache_size = '128GB';
    end if;

    analyze metadata;
    analyze public.spatial_ref_sys;

    raise notice 'refresh polygons';

    -- View of polygon metadata supplied by GDAL crawler, for GSKY
    drop materialized view if exists polygons_tmp cascade;
    create materialized view polygons_tmp as
      select po_hash, po_stamps, po_min_stamp, po_max_stamp,
        ('[' || po_min_stamp || ',' || po_max_stamp || ']')::tstzrange as po_duration,
        po_name, po_polygon
      from (
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
              as stamps
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
      ) t
    ;

    drop materialized view if exists polygons_old cascade;
    alter materialized view if exists polygons rename to polygons_old;
    alter materialized view polygons_tmp rename to polygons;
    drop materialized view if exists polygons_old cascade;

    drop index if exists poi_hash;
    create index poi_hash
      on polygons (po_hash);

    drop index if exists poi_stamp;
    create index poi_stamp
      on polygons (po_min_stamp, po_max_stamp);

    drop index if exists poi_stamps;
    create index poi_stamps
      on polygons using gin (po_stamps);

    drop index if exists poi_duration;
    create index poi_duration
      on polygons using gist (po_duration);

    drop index if exists poi_name;
    create index poi_name
      on polygons (po_name);

    raise notice 'refresh polygon_srids';
    drop materialized view if exists polygon_srids_tmp cascade;
    create materialized view polygon_srids_tmp as
      select
        distinct(public.st_srid(po_polygon))
          as ps_srid
        from
          polygons;

    drop materialized view if exists polygon_srids_old cascade;
    alter materialized view if exists polygon_srids rename to polygons_srids_old;
    alter materialized view polygon_srids_tmp rename to polygon_srids;
    drop materialized view if exists polygon_srids_old cascade;

    for rec in select ci.relname from pg_index i, pg_class ci, pg_class ct where i.indexrelid = ci.oid and i.indrelid = ct.oid and ct.relname = 'polygons' and ci.relname like 'poi\_polygon\_%' loop

      execute format($f$
        drop index if exists %1$s
          $f$, rec.relname
      );

    end loop;

    for rec in select ps_srid as srid from polygon_srids loop

      raise notice 'srid create index poi_polygon_%', rec.srid;

      execute format($f$
        create index if not exists poi_polygon_%1$s on polygons using gist (po_polygon) where public.st_srid(po_polygon) = %1$s
          $f$, rec.srid
      );

    end loop;

    analyze polygons;
    analyze polygon_srids;
    analyze paths;

    return true;
  end
$$;

drop table if exists timestamps_cache cascade;

-- cache for timestamps
create unlogged table timestamps_cache (
  query_id text primary key,
  timestamps jsonb not null
);

create or replace function refresh_caches()
  returns boolean language plpgsql as $$
  begin
    raise notice 'refresh caches';
    truncate timestamps_cache;
    return true;
  end
$$;
