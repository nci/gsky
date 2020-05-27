-- Metadata API

-- Copyright (c) 2017, NCI, Australian National University.
-- All Rights Reserved.

-- ST_Transform will throw an exception if points cannot be meaninfully transformed
-- between projections. Fair enough. Sometimes it's useful to be a bit more lenient,
-- especially when querying across NCI projects with differing data consistency.

\c mas
set role mas;

create or replace function public.ST_SplitDatelineWGS84(polygon geometry)
  returns geometry language plpgsql immutable as $$
  declare
    extended geometry;
    eastern geometry;
    western geometry;
    eastern_hemisphere geometry;
    western_hemisphere geometry;
  begin

    -- When user-supplied bounding box crosses the anti-meridian, some
    -- wgs84 operations assume the polygon goes the other way around the
    -- planet. In order to avoid many false positives, use a multipolygon
    -- split on the anti-meridian.

    if ST_XMin(polygon) < 0 and ST_XMax(polygon) > 0 then

      -- normal bounds
      eastern_hemisphere := ST_MakeEnvelope(
        0, -90, 180, 90
      );

      -- rotated bounds
      western_hemisphere := ST_MakeEnvelope(
        180, -90, 360, 90
      );

      -- ST_ShiftLongitude + remove SRID for intersection
      extended := (
        select
          ST_MakePolygon(
            ST_MakeLine(
              ST_MakePoint(
                case when ST_X(geom) < 0 then 360 + ST_X(geom) else ST_X(geom) end,
                ST_Y(geom)
              )
            )
          )
        from
          ST_DumpPoints(polygon)
            dump
      );

      -- intersection west of dateline
      eastern := ST_SetSRID(
        ST_Intersection(
          extended,
          eastern_hemisphere
        ),
        4326
      );

      -- intersection east of dateline, then un-rotated
      western := ST_SetSRID(
        ST_Translate(
          ST_Intersection(
            extended,
            western_hemisphere
          ),
          -360,
          0
        ),
        4326
      );

      polygon := ST_Collect(
        eastern,
        western
      );

    end if;
    return polygon;
  end
$$;

create or replace function ST_TryTransform(polygon geometry, in_srid integer)
  returns geometry language plpgsql immutable as $$

  declare

    proj4txt    text;

  begin
    proj4txt := (select proj4text from spatial_ref_sys where srid = in_srid limit 1);
    return ST_SetSRID(ST_Transform(polygon, proj4txt), in_srid);
  exception
    when others then
      return null;
  end
$$;

create or replace function ST_TryMakeLine(points geometry[])
  returns geometry language plpgsql immutable as $$

  begin
    return ST_MakeLine(points);
  exception
    when others then
      return null;
  end
$$;

create or replace function ST_TryMakePolygon(line geometry)
  returns geometry language plpgsql immutable as $$

  begin
    return ST_MakePolygon(line);
  exception
    when others then
      return null;
  end
$$;

-- Automatically clip invalid points during transformation

create or replace function ST_LossyTransform(polygon geometry, srid integer)
  returns geometry language sql immutable as $$
    select
      coalesce(

        -- clean transform
        ST_TryTransform(polygon, srid),

        -- or point-by-point transform, removing invalid
        ( select
            coalesce(
              ST_TryMakePolygon(ST_TryMakeLine(array_agg(point))),
              ST_TryMakeLine(array_agg(point)),
              (array_agg(point))[0]
            )
          from (
            select
              ST_TryTransform(geom, srid)
                as point
            from
              ST_DumpPoints(polygon) d
          ) t
          where
            point is not null
        ),

        -- or... bugger...
        ST_GeomFromText('GEOMETRYCOLLECTION EMPTY')
      );
$$;

-- Connection pooling probably in use...

create or replace function mas_reset()
  returns void language plpgsql as $$
  begin
    set work_mem to '32MB';
    perform set_config('search_path', 'public', false);
  end
$$;

-- The MAS database is split into shards, represented using schemas, one
-- per "dataset"... whatever that means in your environment! NCI uses GDATA
-- project codes. An AWS GSKY test used a "vsis3" prefix. The public schema
-- contains multi-schema views to enable cross-dataset queries. The mas_view
-- function sets an API request search_path appropriately.

create or replace function mas_view(gpath text)
  returns text language plpgsql as $$

  declare

    shard text; -- project code

  begin

    shard := coalesce(
      (select sh_code from shards where gpath like concat(sh_path,'%') limit 1), ''
    );

    if octet_length(shard) > 0 and (select true from information_schema.schemata where schema_name = shard) then

      perform set_config('search_path', concat(shard, ', public'), false);

    else

      perform set_config('search_path', 'public', false);

    end if;

    return shard;

  end
$$;

-- Construct an intersection query on "polygons" as a union of statements using
-- each relevant partial index on srid -- ie, how to do indexed postgis
-- intersections with mixed srids.

create or replace function mas_intersect_polygons(
  gpath text,
  bbox geometry,
  namespaces text[],
  time_a timestamptz,
  time_b timestamptz,
  resolution bigint,
  limit_val integer
)
  returns text language plpgsql as $$

  declare

    str text;
    parts text[];
    rec record;
    qstr text;
    limit_str text;

  begin

    if limit_val <= 0 then
      limit_str := '';
    else
      limit_str := format(' limit %1$s ', limit_val);
    end if;

    -- this is the fast path - non-spatial query
    if bbox is null then
      str := format($f$
          select
            distinct pa_path as file
          from
            polygons
          inner join paths
              on po_hash = pa_hash
          where
            (
              -- no times
              %1$L::timestamptz is null

              -- %1$L::timestamptz only
              or (%1$L::timestamptz is not null and %2$L::timestamptz is null
                and %1$L::timestamptz between po_min_stamp and po_max_stamp
                and %1$L::timestamptz = any(po_stamps)
              )

              -- %1$L::timestamptz and %2$L::timestamptz range overlaps stamps
              or (%1$L::timestamptz is not null and %2$L::timestamptz is not null
                and (%1$L::timestamptz - interval '1 second', %2$L::timestamptz + interval '1 second') overlaps (po_min_stamp, po_max_stamp)
              )
            )
            and (%3$L is null
              or po_name = any(%3$L)
            )
            and (%4$L is null
              or po_pixel_x::bigint = %4$L::bigint
            )
            and path_hash(%5$L) = any(pa_parents)
            %6$s

          $f$, time_a, time_b, namespaces, resolution, gpath, limit_str);

      return str;

    end if;

    for rec in select ps_srid as srid from polygon_srids loop

      parts := array_append(parts, format($f$
        (
        select
          po_hash
        from
          polygons
        inner join
          geometries
            on ge_srid = %1$s
        inner join paths
            on po_hash = pa_hash
        where
          st_srid(po_polygon) = %1$s

          and (
            ST_Intersects(po_polygon, ge_geom)
            -- Lossy transform gemoetry may have become a linestring if any points could not be transformed
            or ST_Crosses(po_polygon, ge_geom)
          )

          and (
            -- no times
            %2$L::timestamptz is null

            -- %2$L::timestamptz only
            or (%2$L::timestamptz is not null and %3$L::timestamptz is null
              and %2$L::timestamptz between po_min_stamp and po_max_stamp
              and %2$L::timestamptz = any(po_stamps)
            )

            -- %2$L::timestamptz and %3$L::timestamptz range overlaps stamps
            or (%2$L::timestamptz is not null and %3$L::timestamptz is not null
              and (%2$L::timestamptz - interval '1 second', %3$L::timestamptz + interval '1 second') overlaps (po_min_stamp, po_max_stamp)
            )
          )
          and (%4$L is null
            or po_name = any(%4$L)
          )
          and (%5$L is null
            or po_pixel_x::bigint = %5$L::bigint
          )
          and path_hash(%6$L) = any(pa_parents)
          %7$s )

        $f$, rec.srid, time_a, time_b, namespaces, resolution, gpath, limit_str

      ));

    end loop;

    -- for empty shards
    qstr := coalesce(
      nullif(array_to_string(parts, ' union '), ''),
      'select null::uuid as po_hash limit 0'
    );

    str := format($f$

      with
      geometries as (
        select
          ps_srid
            as ge_srid,
          ST_ConvexHull(ST_LossyTransform(%2$L, ps_srid))
            as ge_geom
        from
          polygon_srids
      )
      select
        distinct(
          path_unhash(po_hash)
        )
          as file
      from (select distinct(po_hash) as po_hash from (%1$s) u %3$s) hashes

      $f$, qstr, bbox, limit_str
    );

    return str;

  end
$$;

-- Find files that contain data within a given bounding polygon, optionally
-- filtered by time, namespace (netcdf variable), and pixel resolution.
-- Include raw metadata from crawlers for each matched file, if requested.

create or replace function mas_intersects(
  gpath      text,
  srs        text, -- EPSG:nnnn
  wkt        text, -- bounding polygon
  n_seg      integer, -- number of segments for polygon segmentation
  time_a     timestamptz, -- time range low
  time_b     timestamptz, -- time range high
  namespace  text[], -- for NetCDF, the variable name
  resolution numeric, -- pixel resolution
  raw_metadata text, -- gdal, pdal
  identity_tol float8, -- distance tolerance considered as same point
  dp_tol       float, -- distance tolerance for Douglas-Peucker algorithm
  limit_val    integer -- limit on number of query rows
)
  returns jsonb language plpgsql as $$
  declare

    srid       integer; -- spatial_ref_sys.srid
    in_geom    geometry; -- temp variable for supplied WKT geometry
    mask       geometry; -- supplied WKT geometry
    segmask    geometry; -- supplied WKT geometry, segmented

    rec        record;
    files      text[];
    result     jsonb;
    qstr       text;

  begin

    if gpath is null then
      raise exception 'invalid search path';
    end if;

    perform mas_reset();
    perform mas_view(gpath);

    if srs is null or wkt is null then
      segmask := null;
    else
      -- &srs=[auth_name]:[auth_srid], eg EPSG:3857
      srid := (
        select spatial_ref_sys.srid
        from spatial_ref_sys
        where srs ~ '^[A-Z]+[:][0-9]+$'
          and auth_name = split_part(srs, ':', 1)
          and auth_srid = split_part(srs, ':', 2)::integer
      );

      if srid is null then
        raise exception 'unknown SRS';
      end if;

      in_geom := ST_GeomFromText(wkt, srid);
      if in_geom is null then
        raise exception 'invalid wkt from user inputs';
      end if;

      if identity_tol is null then
        identity_tol := -1.0;
      end if;

      if dp_tol is null then
        dp_tol := -1.0;
      end if;

      if ST_NPoints(in_geom) > 100 and identity_tol >= 0 and dp_tol >= 0 then
        mask := ST_SimplifyPreserveTopology(ST_RemoveRepeatedPoints(in_geom, identity_tol), dp_tol);
        if mask is null then
          mask := in_geom;
        end if;
      else
        mask := in_geom;
      end if;

      if mask is null then
        raise exception 'invalid WKT';
      end if;

      if n_seg is null then
        n_seg := 2;
      end if;

      -- Intersection occurs in the dataset's original projection. Make
      -- sure the wgs84 bounding box covers roughly the same area after
      -- transformation.
      segmask := ST_Segmentize(
        mask,
        ceil((ST_XMax(mask)-ST_XMin(mask))/n_seg) -- degree lat/lon max segment length
      );
    end if;

    if limit_val is null then
      limit_val := -1;
    end if;

    qstr := mas_intersect_polygons(gpath, segmask, namespace, time_a, time_b, resolution::bigint, limit_val);

    files := array[]::text[];

    for rec in execute qstr loop
      files := array_append(files, rec.file);
    end loop;

    result := jsonb_build_object(
      'files',
      to_jsonb(files)
    );

    -- &metadata=gdal - bundle some raw GDAL metadata for GSKY
    if raw_metadata = 'gdal' then

      result := jsonb_build_object('gdal', coalesce((

        select
          jsonb_agg(dataset)

        from
          metadata

        inner join
          -- Extract and iterate the files discovered above
          jsonb_array_elements_text(result->'files') path
            on md_hash = path_hash(path.value)

        inner join lateral (

          select
            jsonb_build_object(
              'file_path',
              path.value,
              'ds_name',
              geo->>'ds_name',
              'namespace',
              regexp_replace(trim(geo->>'namespace'), '[^a-zA-Z0-9_]', '_', 'g'),
              'array_type',
              geo->'array_type',
              'srs',
              geo->'proj_wkt',
              'geo_transform',
              geo->'geotransform',
              'timestamps',
              geo->'timestamps',
              'polygon',
              geo->>'polygon',
              'overviews',
              geo->'overviews',
              'means',
              geo->'means',
              'sample_counts',
              geo->'sample_counts',
              'nodata',
              geo->'nodata',
              'axes',
              geo->'axes',
              'geo_loc',
              geo->'geo_loc'
            )
              as dataset

          from
            jsonb_array_elements(md_json->'geo_metadata') geo

        ) t
          on true

        where
          md_type = 'gdal'

          and (
            namespace is null
            or dataset->>'namespace' = any(namespace)
          )

      ), '[]'::jsonb));

    end if;

    perform mas_reset();

    return result;
  end
$$;

-- Find all the time stamps overlapping with a given time range
-- The time stamps are filtered by gpath, namespace

create or replace function mas_timestamps(
  gpath      text,        -- file path to search
  time_a     timestamptz, -- time range low
  time_b     timestamptz, -- time range high
  namespace  text[],      -- the variable name
  token      text         -- token that decides if client cache needs refresh 
)
  returns jsonb language plpgsql as $$
  declare
    result     jsonb;
    query_hash text;
    shard      text;
  begin

    if gpath is null then
      raise exception 'invalid search path';
    end if;

    perform mas_reset();
    shard := mas_view(gpath);

    if shard = '' then
      return jsonb_build_object('timestamps', '[]'::jsonb, 'token', '');
    end if;

    query_hash := md5(concat(gpath, coalesce(time_a::text, 'null'),
      coalesce(time_b::text, 'null'), array_to_string(namespace, ',', 'null')));

    if token is not null then
      select jsonb_build_object('timestamps', '[]'::jsonb, 'token', token) into result from timestamps_cache where query_id = query_hash and query_token = token;
      if result is not null then
        return result;
      end if;
    end if;

    select timestamps || jsonb_build_object('token', query_token) into result from timestamps_cache where query_id = query_hash;
    if result is not null then
      return result;
    end if;

    -- By default, we filter out all the future dates
    if time_b is null then
      time_b := (select now());
    end if;

    token := extract(epoch from now())::text;
    result := jsonb_build_object('timestamps', coalesce((

      -- We perform two-stage time range filtering here:
      -- Stage 1: We restrive all the distinct po_stamp tuples that fall into the
      --   time_a and time_b range. Doing so dramatically reduces the computation
      --   of distinct unnest(po_stamps) at the downstream of the query.
      --   If we naively run distinct unnest(po_stamps) directly, we can observe
      --   orders of magnitude of slowdown.
      -- Stage 2: We unnest po_stamps tuples from stage 1 and refine the time range
      --   filtering on a per row basis to form the final result set.

      with stamps_tuple as (
        select distinct po_stamps as po_stamps
        from paths pa
        inner join polygons po
          on po.po_hash = pa.pa_hash
        where path_hash(gpath) = any(pa.pa_parents)
        and (namespace is null or po_name = any(namespace))
        and (time_a is null or po_stamps >= array[time_a])
        and po_stamps <= array[time_b]
      ),
      stamps as (
        select distinct unnest(po_stamps)::timestamptz at time zone 'UTC' as po_stamps
        from stamps_tuple
        order by po_stamps
      )
      select jsonb_agg( to_char(po_stamps, 'YYYY-MM-DD"T"HH24:MI:SS".000Z"')  )
      from stamps
      where (time_a is null or po_stamps >= time_a)
      and po_stamps <= time_b

     ), '[]'::jsonb), 'token', token);

     insert into timestamps_cache (query_id, timestamps, query_token) values (query_hash, result, token)
     on conflict (query_id) do nothing;

     perform mas_reset();
     return result;

  end
$$;

-- Find geospatial and temporal extents 

create or replace function mas_spatial_temporal_extents(
  gpath      text,        -- file path to search
  namespace  text[]       -- the variable name
)
  returns jsonb language plpgsql as $$
  declare
    result jsonb;
    shard text;
    proj4txt text;
  begin
    if gpath is null then
      raise exception 'invalid search path';
    end if;

    perform mas_reset();
    shard := mas_view(gpath);
    if shard = '' then
      return json_build_object(null);
    end if;

    if namespace is null then
      namespace := (select array_agg(po_name)
        from (
          select distinct
            po_name
          from polygons
          inner join paths
            on po_hash = pa_hash
          where public.path_hash(gpath) = any(pa_parents)
          and po_name is not null
          order by po_name
        ) p
      );
    end if;

    proj4txt := (select proj4text from spatial_ref_sys where auth_srid = 3857 limit 1);
    result := (select
      jsonb_build_object(
        'xmin',
        public.ST_XMin(geom),
        'ymax',
        public.ST_YMax(geom),
        'xmax',
        public.ST_XMax(geom),
        'ymin',
        public.ST_YMin(geom),
        'min_stamp',
        min_stamp,
        'max_stamp',
        max_stamp,
        'variables',
        namespace
      )
      from (
        select
          public.ST_Envelope(public.ST_Collect(public.ST_Transform(po_polygon, proj4txt))) geom,
          min(po_min_stamp)::timestamptz at time zone 'UTC' as min_stamp,
          max(po_max_stamp)::timestamptz at time zone 'UTC' as max_stamp
        from polygons
        inner join paths
          on pa_hash = po_hash
        where public.path_hash(gpath) = any(pa_parents)
        and po_name = any(namespace)
      ) g
    );

    perform mas_reset();
    return result;

    end
$$;
