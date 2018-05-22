-- PostgreSQL Utilities

-- Copyright (c) 2016, NCI, Australian National University.

-- Postgres "relkind" as a string
create or replace function relation_type(sname text, rname text)
  returns text language sql as $$
    with list as (
      select c.relkind
      from pg_catalog.pg_class c
      join pg_namespace n
        on n.oid = c.relnamespace
      where n.nspname = sname
        and c.relname = rname
    )
    select
      case
        when relkind = 'r' then 'TABLE'
        when relkind = 'i' then 'INDEX'
        when relkind = 'v' then 'VIEW'
        when relkind = 'm' then 'MATERIALIZED VIEW'
        when relkind = 'f' then 'FOREIGN TABLE'
        else null
      end
    from list;
$$;

-- Drop all functions in a schema
create or replace function drop_functions(schema_name text)
  returns void language plpgsql as $$
  declare
    drop_cmd text;
  begin
    select into drop_cmd
      string_agg(format('drop function %s(%s) cascade;',
        p.oid::regproc, pg_get_function_identity_arguments(p.oid)), e'\n'
      )
    from   pg_proc      p
    join   pg_namespace ns on ns.oid = p.pronamespace
    where  ns.nspname = schema_name
      and  p.oid::regproc::text !~ '^(jam|cstore)';

    if drop_cmd is not null then
      --  raise notice '%', drop_cmd;  -- for debugging
      execute drop_cmd;
    end if;
  end
$$;

-- difference between dates in months
create or replace function age_months(start anyelement, finish anyelement)
  returns integer language sql parallel safe as $$
    select (extract('year' from age(start::timestamptz, date_trunc('month', finish::timestamptz))) * 12
      + extract('month' from age(start::timestamptz, date_trunc('month', finish::timestamptz))))::integer;
$$;

-- set of first-day-of-month dates
create or replace function generate_month_series(start anyelement, finish anyelement)
  returns setof date language sql parallel safe as $$
    select (date_trunc('month', finish::date) + concat(n, ' month')::interval)::date from generate_series(age_months(start, finish), 0) n;
$$;

create or replace function is_current_year(stamp anyelement)
  returns boolean language sql parallel safe as $$
    select date_trunc('year', current_date) = date_trunc('year', stamp::date);
$$;

create or replace function is_current_month(stamp anyelement)
  returns boolean language sql parallel safe as $$
    select date_trunc('month', current_date) = date_trunc('month', stamp::date);
$$;

create or replace function is_current_day(stamp anyelement)
  returns boolean language sql parallel safe as $$
    select stamp::date = current_date;
$$;

create or replace function try_json(str text)
  returns json language plpgsql immutable parallel safe as $$
  begin
    return str::json;
  exception
    when others then
      return null;
  end
$$;

create or replace function try_integer(str text)
  returns integer language plpgsql immutable parallel safe as $$
  begin
    return str::integer;
  exception
    when others then
      return null;
  end
$$;

create or replace function try_inet(str text)
  returns inet language plpgsql immutable parallel safe as $$
  begin
    return str::inet;
  exception
    when others then
      return null;
  end
$$;

create or replace function try_date(str text)
  returns date language plpgsql immutable parallel safe as $$
  begin
    return str::date;
  exception
    when others then
      return null;
  end
$$;

create or replace function try_timestamp(str text)
  returns timestamp language plpgsql immutable parallel safe as $$
  begin
    return str::timestamp;
  exception
    when others then
      return null;
  end
$$;

create or replace function try_timestamptz(str text)
  returns timestamptz language plpgsql immutable parallel safe as $$
  begin
    return str::timestamptz;
  exception
    when others then
      return null;
  end
$$;

create or replace function notnull(item anyelement)
  returns anyelement language plpgsql immutable parallel safe as $$
  begin
    if item is null then
      raise exception null_value_not_allowed;
    end if;
    return item;
  end
$$;
