-- Metadata

-- Copyright (c) 2016, NCI, Australian National University.
-- All Rights Reserved.

-- This is the entry point for loading an MAS "public" schema:
--
-- psql -f schema.sql
--
-- Each shard schema (see shard.sql and "shards" table) is then loaded as required:
--
-- psql -f fr5.sql

set role postgres;
\c postgres

-- Atomic DDL can't handle a database drop, so kill off active clients
update pg_database set datallowconn = 'false' where datname = 'mas';
select pg_terminate_backend(pid) from pg_stat_activity where datname = 'mas';

drop database if exists mas;

do $$
  begin

    if (select true from pg_roles where rolname = 'mas') then
      drop owned by mas cascade;
      drop role mas;
    end if;

    if (select true from pg_roles where rolname = 'api') then
      drop owned by api cascade;
      drop role api;
    end if;

  end
$$;

create role mas with password null;
create database mas owner mas;
grant all privileges on database mas to mas;

create role api with password null login;
grant connect on database mas to api;

\c mas

alter default privileges for role postgres in schema public grant select on tables to public;
alter default privileges for role mas in schema public grant select on tables to public;

set search_path to public;
set client_min_messages to warning;

create extension postgis;
grant all on spatial_ref_sys to mas;

-- Postgres has no built-in operators for indexing UUID using GIN
create operator class _uuid_ops default
  for type _uuid using gin as
  operator 1 &&(anyarray, anyarray),
  operator 2 @>(anyarray, anyarray),
  operator 3 <@(anyarray, anyarray),
  operator 4 =(anyarray, anyarray),
  function 1 uuid_cmp(uuid, uuid),
  function 2 ginarrayextract(anyarray, internal, internal),
  function 3 ginqueryarrayextract(anyarray, internal, smallint, internal, internal, internal, internal),
  function 4 ginarrayconsistent(internal, smallint, anyarray, integer, internal, internal, internal, internal),
  storage uuid;

set role mas;

\i util.sql

create type path_type as enum ('file', 'directory', 'link');

-- List of projects and their top-level gdata paths
create table shards (
  sh_code text,
  sh_path text
);

create unique index shi_code
  on shards (sh_code);

create unique index shi_path
  on shards (sh_path);

-- SRS descriptions found on gdata that don't match official EPSG
-- codes in the PostGIS distribution
create table nci_spatial_ref_sys (
  srid integer not null,
  auth_name text,
  srtext text,
  proj4text text
);

-- Start our custom SRID values at 100000, well above the EPSG range
create sequence nci_spatial_ref_sys_srid_seq
  start with 100000
  increment by 1
  no minvalue
  no maxvalue
  cache 1;

alter sequence nci_spatial_ref_sys_srid_seq
  owned by nci_spatial_ref_sys.srid;

alter table only nci_spatial_ref_sys alter column srid
  set default nextval('nci_spatial_ref_sys_srid_seq'::regclass);

-- Create views combining the shards to allow cross-project searches. These
-- currently use UNION which Postgres is able to prune fairly effectively.
-- Compared to table inheritance, this approach allows a shard to be a
-- foreign table if we want to scale horizontally, or to be asynchronously
-- reloaded/altered without a storm of locking.
create function create_views()
  returns boolean language plpgsql as $$

  begin

    drop materialized view if exists public.paths_common cascade;

    execute format($f$
      create materialized view public.paths_common as
        select
          t.*,
          (select array_agg(md5(p)::uuid) from public.parent_paths(t.pa_path) p) as pa_parents
        from (
          select
            pa_hash,
            max(pa_ingested) as pa_ingested,
            (array_agg(pa_type))[1] as pa_type,
            (array_agg(pa_path))[1] as pa_path
          from
            (%1$s) t
          group by
            pa_hash
        ) t
    $f$,
      (select string_agg(concat('select * from ', sh_code, '.paths where length(pa_path) <= 8'), ' union all ') from public.shards)
    );

    drop view if exists public.paths cascade;

    execute format($f$
      create view public.paths as select t.* from (%1$s union all (select * from public.paths_common)) t $f$,
        (select string_agg(concat('select * from ', sh_code, '.paths where pa_hash not in (select pa_hash from public.paths_common)'), ' union all ') from public.shards)
    );

    drop materialized view if exists public.directories_common cascade;

    execute format($f$
      create materialized view public.directories_common as
        select
          t.*
        from (
          select
            di_hash,
            di_parent,
            di_name,
            null::timestamptz
              as di_ctime,
            null::timestamptz
              as di_mtime,
            null::timestamptz
              as di_atime,
            null::text
              as di_mode,
            null::text
              as di_inode,
            null::text
              as di_uid,
            null::text
              as di_gid,
            null::text
              as di_user,
            null::text
              as di_group,
            sum(di_count)
              as di_count,
            sum(di_size)
              as di_size
          from
            (%1$s) t
          group by
            di_hash,
            di_parent,
            di_name
        ) t
    $f$,
      (select string_agg(concat('select d.* from ', sh_code, '.directories d where di_hash in (select pa_hash from public.paths_common)'), ' union all ') from public.shards)
    );

    drop view if exists public.directories;

    execute format($f$
      create view public.directories as select t.* from (%1$s union all (select * from public.directories_common)) t $f$,
        (select string_agg(concat('select d.* from ', sh_code, '.directories d join ', sh_code, '.paths on di_hash = pa_hash where pa_hash not in (select pa_hash from public.paths_common)'), ' union all ') from public.shards)
    );

    drop view if exists public.files;

    execute format($f$
      create view public.files as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select f.* from ', sh_code, '.files f join ', sh_code, '.paths on fi_hash = pa_hash where pa_hash not in (select pa_hash from public.paths_common)'), ' union all ') from public.shards)
    );

    drop view if exists public.links;

    execute format($f$
      create view public.links as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select l.* from ', sh_code, '.links l join ', sh_code, '.paths on li_hash = pa_hash where pa_hash not in (select pa_hash from public.paths_common)'), ' union all ') from public.shards)
    );

    drop view if exists public.polygons;

    execute format($f$
      create view public.polygons as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select p.* from ', sh_code, '.polygons p join ', sh_code, '.paths on po_hash = pa_hash where pa_hash not in (select pa_hash from public.paths_common)'), ' union all ') from public.shards)
    );

    drop view if exists public.polygon_srids;

    execute format($f$
      create view public.polygon_srids as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select p.* from ', sh_code, '.polygon_srids p'), ' union all ') from public.shards)
    );

    drop view if exists public.paths;

    execute format($f$
      create view public.paths as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select p.* from ', sh_code, '.paths p'), ' union all ') from public.shards)
    );

    drop view if exists public.metadata;

    execute format($f$
      create view public.metadata as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select m.* from ', sh_code, '.metadata m'), ' union all ') from public.shards)
    );

    drop view if exists public.netcdf;

    execute format($f$
      create view public.netcdf as select t.* from (%1$s) t $f$,
        (select string_agg(concat('select n.* from ', sh_code, '.netcdf n'), ' union all ') from public.shards)
    );

    return true;
  end
$$;

create function refresh_views()
  returns boolean language plpgsql as $$

  begin

    perform public.create_views();

    return true;
  end
$$;

-- Split a path to a list of all parent paths
create function parent_paths(dir text)

-- If input:
-- /g/data3/abc/blah

-- Then output:
-- /g
-- /g/data3
-- /g/data3/abc

  returns setof text language sql immutable as $$
    with
      parts as (
        select
          row_number() over ()
            as n,
          t.part
        from (
          select
            unnest(string_to_array(trim('/' from dir), '/')) part
        ) t
      )
    select
      path
    from (
      select
        concat('/', string_agg(b.part,'/'))
          as path
      from
        parts a
      inner join
        parts b
          on a.n >= b.n
      group by
        a.n
    ) t
    where
      path <> dir
    ;
$$;

-- Convert a UUID to a path
create function path_unhash(hash uuid)
  returns text language plpgsql immutable as $$
  begin
    return (select pa_path from paths where pa_hash = hash);
  end
$$;

-- Convert a path to a UUID
create function path_hash(path text)
  returns uuid language sql immutable as $$
    select md5(path)::uuid;
$$;

create function path_resolve(path text)
  returns text language sql immutable as $$
    select regexp_replace(path, '^/g/data[0-9]*/[^/]+', sh_path) from public.shards where sh_path like concat('%', split_part(path, '/', 4));
$$;

create function path_absolute(path text, relative text)
  returns text language sql immutable as $$
    select case when path like '.%' then concat(relative, '/', substr(path, 3)) else path end;
$$;
