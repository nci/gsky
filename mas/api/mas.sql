-- Metadata API

-- Copyright (c) 2017, NCI, Australian National University.
-- All Rights Reserved.

-- ST_Transform will throw an exception if points cannot be meaninfully transformed
-- between projections. Fair enough. Sometimes it's useful to be a bit more lenient,
-- especially when querying across NCI projects with differing data consistency.

\c mas
set role mas;

create or replace function ST_SplitDatelineWGS84(polygon geometry)
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
    set lock_timeout to '10s';
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

-- Find files that contain data within a given bounding polygon, optionally
-- filtered by time, namespace (netcdf variable), etc.
-- Include raw metadata from crawlers for each matched file, if requested.

create or replace function mas_intersects(
  gpath      text,
  srs        text, -- EPSG:nnnn
  wkt        text, -- bounding polygon
  n_seg      integer, -- number of segments for polygon segmentation
  time_a     timestamptz, -- time range low
  time_b     timestamptz, -- time range high
  namespace  text[], -- for NetCDF, the variable name
  raw_metadata text, -- gdal, pdal
  identity_tol float8, -- distance tolerance considered as same point
  dp_tol       float, -- distance tolerance for Douglas-Peucker algorithm
  limit_val    integer -- limit on number of query rows
)
  returns jsonb language plpgsql as $$
  declare

    shard      text;
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
    shard := mas_view(gpath);

    if shard = '' then
      return jsonb_build_object('gdal', '[]'::jsonb);
    end if;

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

      if identity_tol >= 0 and dp_tol >= 0 and ST_NPoints(in_geom) > 100 then
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

    if limit_val <= 0 then
      limit_val := null;
    end if;

    if raw_metadata = 'gdal' then
      if segmask is not null then
        return shard_intersect_polygons(gpath, segmask, namespace, time_a, time_b, limit_val);
      else
        return shard_intersect_times(gpath, namespace, time_a, time_b, limit_val);
      end if;
    end if;

    perform mas_reset();

    return result;
  end
$$;

create or replace function codegen_shard_intersect_times()
  returns text language plpgsql as $$
  declare
    str text;
  begin
    str := $f$
      select distinct po_hash
      from
        polygons
      inner join paths
          on po_hash = pa_hash
      where
        (
          -- time_a and time_b range overlaps stamps
          (time_a is not null and time_b is not null
            and ('[' || time_a - interval '1 second' || ',' || time_b + interval '1 second' || ']')::tstzrange && po_duration
          )

          -- time_a only
          or (time_a is not null and time_b is null
            and time_a between po_min_stamp and po_max_stamp
            and time_a = any(po_stamps)
          )

          -- no times
          or time_a is null
        )
        and (namespaces is null
          or po_name = any(namespaces)
        )
        and path_hash(gpath) = any(pa_parents)
        limit limit_val

      $f$;

    str := format(codegen_gdal_json(), str);

    return format($f$

      create or replace function shard_intersect_times(
          gpath text,
          namespaces text[],
          time_a timestamptz,
          time_b timestamptz,
          limit_val integer
      )
        returns jsonb language plpgsql as $ff$
        begin
          return %1$s
        end
      $ff$;

    $f$, str);

  end
$$;

create or replace function codegen_shard_intersect_polygons()
  returns text language plpgsql as $$
  declare
    str text;
    parts text[];
    rec record;
    qstr text;
  begin

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

          and ST_Intersects(po_polygon, ge_geom)

          and (
             -- time_a and time_b range overlaps stamps
            (time_a is not null and time_b is not null
              and ('[' || time_a - interval '1 second' || ',' || time_b + interval '1 second' || ']')::tstzrange && po_duration
            )

            -- time_a only
            or (time_a is not null and time_b is null
              and time_a::timestamptz between po_min_stamp and po_max_stamp
              and time_a::timestamptz = any(po_stamps)
            )

            -- no times
            or time_a is null
          )
          and (
            po_name = any(namespaces)
            or namespaces is null
          )
          and path_hash(gpath) = any(pa_parents)
          limit limit_val )

        $f$, rec.srid

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
          ST_ConvexHull(ST_LossyTransform(bbox, ps_srid))
            as ge_geom
        from
          polygon_srids
      )
      select po_hash
      from (select po_hash as po_hash from (%1$s) u limit limit_val) hashes

      $f$, qstr
    );

    str := format(codegen_gdal_json(), str);

    return format($f$

      create or replace function shard_intersect_polygons(
          gpath text,
          bbox geometry,
          namespaces text[],
          time_a timestamptz,
          time_b timestamptz,
          limit_val integer
      )
        returns jsonb language plpgsql as $ff$
        begin
          return %1$s
        end
      $ff$;

    $f$, str);

  end
$$;

create or replace function codegen_gdal_json()
  returns text language plpgsql as $$
  begin

    return $f$

      jsonb_build_object('gdal', coalesce((
        select
          jsonb_agg(dataset)

        from
          metadata

        inner join (
          %1$s
        ) h
          on po_hash = md_hash

        inner join lateral (

          select
            jsonb_build_object(
              'file_path',
              md_json->>'filename',
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
            namespaces is null
            or dataset->>'namespace' = any(namespaces)
          )

      ), '[]'::jsonb));

    $f$;
  end
$$;

create or replace function mas_refresh_codegens()
  returns boolean language plpgsql as $$
  declare
    shard text;
    rec record;
  begin
    perform mas_reset();

    for rec in select sh_path as gpath from shards loop
      shard := mas_view(rec.gpath);
      if shard = '' then
        continue;
      end if;

      raise notice 'refresh codegen for shard %', shard;
      execute codegen_shard_intersect_polygons();
      execute codegen_shard_intersect_times();

      perform mas_reset();
    end loop;

    perform mas_reset();
    return true;
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

    if token is not null and token = query_hash then
      select jsonb_build_object('timestamps', '[]'::jsonb, 'token', query_hash) into result from timestamps_cache where query_id = query_hash;
      if result is not null then
        return result;
      end if;
    end if;

    select timestamps || jsonb_build_object('token', query_hash) into result from timestamps_cache where query_id = query_hash;
    if result is not null then
      return result;
    end if;

    -- By default, we filter out all the future dates
    if time_b is null then
      time_b := (select now());
    end if;

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

     ), '[]'::jsonb), 'token', query_hash);

     insert into timestamps_cache (query_id, timestamps) values (query_hash, result)
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
      return '{}'::jsonb;
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

create or replace function mas_generate_layers (
  gpath text
)
  returns jsonb language plpgsql as $$
  declare
    result jsonb;
    shard text;
    namespaces jsonb;
  begin
    if gpath is null then
      raise exception 'invalid search path';
    end if;

    perform mas_reset();
    shard := mas_view(gpath);
    if shard = '' then
      return '{}'::jsonb;
    end if;

    namespaces := mas_list_namespaces(gpath);

    result := jsonb_build_object(
      'layers',
      coalesce((select array_agg(layer)
      from
        (select jsonb_build_object(
          'title',
          t1.ns,
          'name',
          t1.ns,
          'time_generator',
          'mas',
          'data_source',
          gpath,
          'rgb_products',
          array_fill(t1.ns, ARRAY[1])
          ) || case when t2.axis is not null then
                jsonb_build_object('axes', t2.axis)
              else '{}'::jsonb end as layer
          from (
            select jsonb_array_elements(namespaces->'namespaces') as ns
          ) t1
          left join lateral (
            select axis
            from mas_list_namespace_axes(gpath, namespaces) ax(ns jsonb, axis jsonb[])
            where ax.ns = t1.ns
          ) t2 on true
        ) t3
      ), '{}'::jsonb[])
    );

    perform mas_reset();
    return result;

  end
$$;

create or replace function mas_list_namespaces (
  gpath text
)
  returns jsonb language plpgsql as $$
  declare
    result jsonb;
    shard text;
    gpath_hash uuid;
  begin
    if gpath is null then
      raise exception 'invalid search path';
    end if;

    gpath := '/' || trim(gpath, '/');
    gpath_hash := public.path_hash(gpath);

    result := jsonb_build_object(
      'namespaces',
      coalesce((select jsonb_agg(po_name)
      from (
        select distinct po_name
        from polygons po
         inner join (
           select pa_hash
           from paths pa
           where gpath_hash = pa.pa_parents[array_length(pa_parents, 1)]
         ) t1 on t1.pa_hash = po.po_hash
         order by po_name
      ) t2
      ), '[]'::jsonb)
    );

    return result;
  end
$$;

create or replace function mas_list_namespace_axes (
  gpath text,
  namespaces jsonb
)
  returns setof record language plpgsql as $$
  begin
      return query
      select t1.ns, array_agg(axis) over (partition by t1.ns) axis
      from (
        select jsonb_array_elements(namespaces->'namespaces') as ns
      ) t1
      inner join lateral
      (
        select jsonb_build_object (
          'name',
          name,
          'values',
          (select array_agg(value) from jsonb_array_elements_text(params))
        ) axis
        from (
          select distinct t3.axes->'name' as name,
            first_value(t3.axes->'params') over (partition by t3.axes->>'name' order by jsonb_array_length(t3.axes->'params') desc) as params
          from (
            select jsonb_array_elements(geo->'axes') as axes
            from (
              select jsonb_array_elements(md.md_json->'geo_metadata') geo
              from paths pa
              inner join metadata md
                on md.md_hash = pa.pa_hash
              where public.path_hash(gpath) = any(pa.pa_parents)
            ) t2
            where geo->'namespace' = t1.ns
          ) t3
          where t3.axes->>'params' is not null
          and jsonb_array_length(t3.axes->'params') > 0
        ) t4
      ) t5 on true;

  end
$$;

create or replace function mas_list_root_gpath ()
  returns jsonb language plpgsql as $$
  declare
    result jsonb;
  begin
    result := jsonb_build_object(
      'sub_paths',
      coalesce((select jsonb_agg(sh_path)
        from (select sh_path
            from shards
            order by sh_path
          ) t
        ), '[]'::jsonb)
      );

    return result;

  end
$$;

create or replace function mas_list_sub_gpath (
  gpath text
)
  returns jsonb language plpgsql as $$
  declare
    shard text;
    sub_path_result jsonb;
    namespace_result jsonb;
    gpath_root_result jsonb;
    gpath_root text;
    gpath_depth int;
    gpath_hash uuid;
  begin
    perform mas_reset();
    shard := mas_view(gpath);
    if shard = '' then
      return '{}'::jsonb;
    end if;

    gpath := '/' || trim(gpath, '/');
    gpath_depth := length(gpath) - length(replace(gpath, '/', ''));
    gpath_hash := public.path_hash(gpath);

    sub_path_result := jsonb_build_object(
      'sub_paths',
      coalesce((select jsonb_agg(substr(t.sub_path, length(gpath)+1))
        from (
          select t2.sub_path from (
            select distinct on (pa_parents[gpath_depth+1]) pa_parents[gpath_depth+1] as path_hash, pa_path
            from paths where gpath_hash = pa_parents[gpath_depth]
          ) t1
          inner join lateral (
            select sub_path, sub_path_hash
            from (
              select path as sub_path, (md5(path)::uuid) as sub_path_hash
              from public.parent_paths(t1.pa_path) path
            ) t
            where sub_path_hash = t1.path_hash
            limit 1
          )  t2 on true
          order by t2.sub_path
        ) t
      ), '[]'::jsonb)
    );

    namespace_result := jsonb_build_object(
      'has_namespaces',
      coalesce((select true
      from polygons po
        inner join (
          select pa_hash
          from paths pa
          where gpath_hash = pa.pa_parents[array_length(pa_parents, 1)]
        ) t1 on t1.pa_hash = po.po_hash
        limit 1
      ), false)
    );

    gpath_root := (select sh_path from public.shards where gpath like concat(sh_path,'%') limit 1);
    gpath_root_result := jsonb_build_object(
      'gpath_root',
      gpath_root
    );

    perform mas_reset();
    return sub_path_result || namespace_result || gpath_root_result;

  end
$$;

select mas_refresh_codegens();
