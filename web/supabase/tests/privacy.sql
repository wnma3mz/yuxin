begin;

do $$
begin
  if has_schema_privilege('anon', 'private', 'usage') then
    raise exception 'anon unexpectedly has private schema usage';
  end if;
  if has_table_privilege('anon', 'private.public_contributions', 'select')
    or has_table_privilege('anon', 'private.public_contributions', 'insert')
    or has_table_privilege('anon', 'private.public_messages', 'select') then
    raise exception 'anon unexpectedly has direct raw table privileges';
  end if;
  if not has_function_privilege('anon', 'public.submit_public_data(integer,integer,integer,bigint,integer,text)', 'execute') then
    raise exception 'anon cannot call submit_public_data';
  end if;
  if not has_function_privilege('anon', 'public.submit_public_message(text,text)', 'execute') then
    raise exception 'anon cannot call submit_public_message';
  end if;
  if not has_function_privilege('anon', 'public.get_public_dashboard()', 'execute')
    or not has_function_privilege('anon', 'public.get_public_messages(integer)', 'execute') then
    raise exception 'anon cannot call public read functions';
  end if;
  if exists (
    select 1
    from pg_catalog.pg_proc as p
    join pg_catalog.pg_namespace as n on n.oid = p.pronamespace
    where n.nspname = 'public'
      and p.proname in ('submit_public_data', 'submit_public_message', 'get_public_dashboard', 'get_public_messages')
      and has_function_privilege('authenticated', p.oid, 'execute')
  ) then
    raise exception 'authenticated or PUBLIC unexpectedly can execute a dashboard RPC';
  end if;
  if has_function_privilege('anon', 'private.released_count(bigint)', 'execute') then
    raise exception 'anon unexpectedly can call private released_count';
  end if;
end;
$$;

set local role anon;

select public.submit_public_data(
  8123,
  493,
  5,
  12345,
  30
);

select public.submit_public_data(
  8100,
  480,
  5,
  12000,
  30,
  repeat('a', 64)
);

select public.submit_public_data(
  12600,
  420,
  4,
  null,
  24,
  repeat('a', 64)
);

select public.submit_public_message(
  'encourage',
  '今天也辛苦了'
);

select public.get_public_dashboard();

reset role;

do $$
declare
  contribution private.public_contributions%rowtype;
  editable_count integer;
begin
  select * into contribution
  from private.public_contributions
  where edit_credential_hash is null
  order by id desc
  limit 1;

  if contribution.monthly_salary_cny <> 8100
    or contribution.daily_work_minutes <> 480
    or contribution.savings_cny <> 12000 then
    raise exception 'server-side precision reduction failed: %', row_to_json(contribution);
  end if;

  select count(*) into editable_count
  from private.public_contributions
  where edit_credential_hash is not null;
  if editable_count <> 1 then
    raise exception 'correction credential created duplicate contributions: %', editable_count;
  end if;
  select * into contribution
  from private.public_contributions
  where edit_credential_hash is not null;
  if contribution.monthly_salary_cny <> 12600
    or contribution.daily_work_minutes <> 420
    or contribution.workdays_per_week <> 4
    or contribution.savings_cny is not null
    or contribution.retirement_years_remaining <> 24
    or octet_length(contribution.edit_credential_hash) <> 32 then
    raise exception 'credential correction failed: %', row_to_json(contribution);
  end if;

  if exists (
    select 1
    from public.get_public_messages(24)
    where text = '今天也辛苦了'
  ) then
    raise exception 'pending message was exposed publicly';
  end if;

  if (select (public.get_public_dashboard()->>'totalSubmissions')::integer) <> 2 then
    raise exception 'same-day contributions were not included in the public dashboard';
  end if;
end;
$$;

insert into private.public_contributions (
  submitted_on,
  monthly_salary_cny,
  daily_work_minutes,
  workdays_per_week,
  savings_cny,
  retirement_years_remaining
)
select
  (pg_catalog.timezone('utc', now()))::date - 1,
  case when sample <= 4 then 2000 else 8000 end,
  480,
  5,
  sample * 10000,
  20
from generate_series(1, 10) as sample;

do $$
declare
  dashboard jsonb := public.get_public_dashboard();
begin
  if (dashboard->>'totalSubmissions')::integer <> 12 then
    raise exception 'public total should include every contribution: %', dashboard;
  end if;
  if (dashboard #>> '{distributions,salary,0,count}')::integer <> 0 then
    raise exception 'salary bucket below five samples was not suppressed: %', dashboard;
  end if;
  if (dashboard #>> '{distributions,salary,3,count}')::integer <> 5 then
    raise exception 'eligible salary bucket was not released in groups of five: %', dashboard;
  end if;
  if (dashboard #>> '{metrics,medianSalaryCny}')::integer % 500 <> 0
    or (dashboard #>> '{metrics,medianDailyWorkMinutes}')::integer % 30 <> 0
    or (dashboard #>> '{metrics,medianHourlyIncomeCny}')::integer % 5 <> 0 then
    raise exception 'public medians were not coarsened to fixed units: %', dashboard;
  end if;
  if (dashboard #>> '{metrics,layFlatSampleCount}')::integer <> 10
    or (dashboard #>> '{metrics,medianLayFlatDailyCny}')::integer < 0 then
    raise exception 'public lay-flat aggregate is missing or invalid: %', dashboard;
  end if;
  if jsonb_array_length(dashboard #> '{matrices,workValue}') <> 4
    or jsonb_array_length(dashboard #> '{matrices,chillIndex}') <> 4 then
    raise exception 'public matrices do not have four privacy-safe quadrants: %', dashboard;
  end if;
  if (dashboard #>> '{matrices,workValue,0,count}')::integer <> 5
    or (dashboard #>> '{matrices,chillIndex,1,count}')::integer <> 5 then
    raise exception 'matrix quadrants were not released in groups of five: %', dashboard;
  end if;
end;
$$;

do $$
declare
  before_extra_sample jsonb := public.get_public_dashboard();
  after_extra_sample jsonb;
begin
  insert into private.public_contributions (
    submitted_on,
    monthly_salary_cny,
    daily_work_minutes,
    workdays_per_week
  ) values (
    (pg_catalog.timezone('utc', now()))::date - 1,
    100000,
    900,
    7
  );

  after_extra_sample := public.get_public_dashboard();
  if (after_extra_sample->>'totalSubmissions')::integer
      <> (before_extra_sample->>'totalSubmissions')::integer + 1 then
    raise exception 'a new contribution did not increment the public dashboard: before %, after %',
      before_extra_sample,
      after_extra_sample;
  end if;
end;
$$;

rollback;
