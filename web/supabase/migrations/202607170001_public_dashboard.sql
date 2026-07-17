create schema if not exists private;

revoke all on schema private from public, anon, authenticated;

create table private.public_contributions (
  id bigint generated always as identity primary key,
  submitted_on date not null default (timezone('utc', now()))::date,
  monthly_salary_cny integer not null
    check (monthly_salary_cny between 100 and 10000000 and monthly_salary_cny % 100 = 0),
  daily_work_minutes smallint not null
    check (daily_work_minutes between 60 and 960 and daily_work_minutes % 30 = 0),
  workdays_per_week smallint not null
    check (workdays_per_week between 1 and 7),
  savings_cny bigint
    check (savings_cny is null or (savings_cny between 0 and 1000000000000 and savings_cny % 1000 = 0)),
  retirement_years_remaining smallint
    check (retirement_years_remaining is null or retirement_years_remaining between 0 and 82)
);

comment on table private.public_contributions is
  'Anonymous coarse-grained values. No user identifier, IP, user agent, or exact timestamp.';

create table private.public_messages (
  id bigint generated always as identity primary key,
  submitted_on date not null default (timezone('utc', now()))::date,
  kind text not null check (kind in ('advice', 'rant', 'wish', 'encourage')),
  body text not null check (char_length(body) between 1 and 80),
  status text not null default 'pending' check (status in ('pending', 'approved', 'rejected'))
);

comment on table private.public_messages is
  'Moderated anonymous messages. Deliberately has no foreign key or identifier linking it to contribution values.';

alter table private.public_contributions enable row level security;
alter table private.public_messages enable row level security;

revoke all on table private.public_contributions, private.public_messages from public, anon, authenticated;
revoke all on sequence private.public_contributions_id_seq, private.public_messages_id_seq from public, anon, authenticated;

create or replace function private.released_count(p_count bigint)
returns bigint
language sql
immutable
parallel safe
set search_path = ''
as $$
  select floor(greatest(p_count, 0) / 5.0)::bigint * 5;
$$;

revoke all on function private.released_count(bigint) from public, anon, authenticated;

create or replace function public.submit_public_data(
  p_monthly_salary_cny integer,
  p_daily_work_minutes integer,
  p_workdays_per_week integer,
  p_savings_cny bigint,
  p_retirement_years_remaining integer
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  normalized_salary integer;
  normalized_minutes smallint;
  normalized_savings bigint;
begin
  if p_monthly_salary_cny is null or p_monthly_salary_cny not between 100 and 10000000 then
    raise exception 'monthly salary is out of range' using errcode = '22023';
  end if;
  if p_daily_work_minutes is null or p_daily_work_minutes not between 60 and 960 then
    raise exception 'daily work minutes are out of range' using errcode = '22023';
  end if;
  if p_workdays_per_week is null or p_workdays_per_week not between 1 and 7 then
    raise exception 'workdays per week are out of range' using errcode = '22023';
  end if;
  if p_savings_cny is not null and p_savings_cny not between 0 and 1000000000000 then
    raise exception 'savings are out of range' using errcode = '22023';
  end if;
  if p_retirement_years_remaining is not null and p_retirement_years_remaining not between 0 and 82 then
    raise exception 'retirement years are out of range' using errcode = '22023';
  end if;
  normalized_salary := (round(p_monthly_salary_cny::numeric / 100) * 100)::integer;
  normalized_minutes := (round(p_daily_work_minutes::numeric / 30) * 30)::smallint;
  normalized_savings := case when p_savings_cny is null then null else (round(p_savings_cny::numeric / 1000) * 1000)::bigint end;

  insert into private.public_contributions (
    monthly_salary_cny,
    daily_work_minutes,
    workdays_per_week,
    savings_cny,
    retirement_years_remaining
  ) values (
    normalized_salary,
    normalized_minutes,
    p_workdays_per_week,
    normalized_savings,
    p_retirement_years_remaining
  );

end;
$$;

create or replace function public.submit_public_message(
  p_message_kind text,
  p_message_text text
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  normalized_message text := nullif(btrim(p_message_text), '');
begin
  if p_message_kind not in ('advice', 'rant', 'wish', 'encourage') then
    raise exception 'message kind is invalid' using errcode = '22023';
  end if;
  if normalized_message is null
    or char_length(normalized_message) > 80
    or normalized_message ~ '[[:cntrl:]]'
    or lower(normalized_message) like '%http://%'
    or lower(normalized_message) like '%https://%'
    or lower(normalized_message) like '%www.%' then
    raise exception 'message text is invalid' using errcode = '22023';
  end if;

  insert into private.public_messages (kind, body)
  values (p_message_kind, normalized_message);
end;
$$;

create or replace function public.get_public_dashboard()
returns jsonb
language sql
stable
security definer
set search_path = ''
as $$
  with eligible as (
    select c.*, row_number() over (order by c.id) as release_order
    from private.public_contributions as c
    where c.submitted_on < (pg_catalog.timezone('utc', now()))::date
  ),
  release_window as (
    select floor(count(*) / 10.0)::bigint * 10 as release_size
    from eligible
  ),
  visible as (
    select e.*
    from eligible as e
    cross join release_window as w
    where e.release_order <= w.release_size
  )
  select jsonb_build_object(
    'totalSubmissions', floor(count(*) / 10.0)::bigint * 10,
    'updatedDate', max(c.submitted_on)::text,
    'metrics', jsonb_build_object(
      'medianSalaryCny', (round(percentile_cont(0.5) within group (order by c.monthly_salary_cny) / 500) * 500)::bigint,
      'medianDailyWorkMinutes', (round(percentile_cont(0.5) within group (order by c.daily_work_minutes) / 30) * 30)::integer,
      'medianHourlyIncomeCny', (round(percentile_cont(0.5) within group (
        order by c.monthly_salary_cny * 12.0 / ((c.daily_work_minutes / 60.0) * c.workdays_per_week * 52.0)
      ) / 5) * 5)::integer,
      'medianLayFlatDailyCny', case
        when count(*) filter (where c.savings_cny is not null and c.retirement_years_remaining > 0) >= 5
        then round(percentile_cont(0.5) within group (
          order by c.savings_cny / (c.retirement_years_remaining * 365.2425)
        ) filter (where c.savings_cny is not null and c.retirement_years_remaining > 0))::integer
        else null
      end,
      'salarySampleCount', private.released_count(count(*)),
      'savingsSampleCount', private.released_count(count(c.savings_cny)),
      'retirementSampleCount', private.released_count(count(c.retirement_years_remaining)),
      'layFlatSampleCount', private.released_count(count(*) filter (
        where c.savings_cny is not null and c.retirement_years_remaining > 0
      ))
    ),
    'distributions', jsonb_build_object(
      'salary', jsonb_build_array(
        jsonb_build_object('label', '3千以下', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny < 3000))),
        jsonb_build_object('label', '3–5千', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny >= 3000 and c.monthly_salary_cny < 5000))),
        jsonb_build_object('label', '5–8千', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny >= 5000 and c.monthly_salary_cny < 8000))),
        jsonb_build_object('label', '8千–1.2万', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny >= 8000 and c.monthly_salary_cny < 12000))),
        jsonb_build_object('label', '1.2–2万', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny >= 12000 and c.monthly_salary_cny < 20000))),
        jsonb_build_object('label', '2–3万', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny >= 20000 and c.monthly_salary_cny < 30000))),
        jsonb_build_object('label', '3万以上', 'count', private.released_count(count(*) filter (where c.monthly_salary_cny >= 30000)))
      ),
      'workHours', jsonb_build_array(
        jsonb_build_object('label', '6小时以下', 'count', private.released_count(count(*) filter (where c.daily_work_minutes < 360))),
        jsonb_build_object('label', '6–8小时', 'count', private.released_count(count(*) filter (where c.daily_work_minutes >= 360 and c.daily_work_minutes < 480))),
        jsonb_build_object('label', '8–10小时', 'count', private.released_count(count(*) filter (where c.daily_work_minutes >= 480 and c.daily_work_minutes < 600))),
        jsonb_build_object('label', '10–12小时', 'count', private.released_count(count(*) filter (where c.daily_work_minutes >= 600 and c.daily_work_minutes < 720))),
        jsonb_build_object('label', '12小时以上', 'count', private.released_count(count(*) filter (where c.daily_work_minutes >= 720)))
      ),
      'savings', jsonb_build_array(
        jsonb_build_object('label', '1万以下', 'count', private.released_count(count(*) filter (where c.savings_cny < 10000))),
        jsonb_build_object('label', '1–5万', 'count', private.released_count(count(*) filter (where c.savings_cny >= 10000 and c.savings_cny < 50000))),
        jsonb_build_object('label', '5–10万', 'count', private.released_count(count(*) filter (where c.savings_cny >= 50000 and c.savings_cny < 100000))),
        jsonb_build_object('label', '10–30万', 'count', private.released_count(count(*) filter (where c.savings_cny >= 100000 and c.savings_cny < 300000))),
        jsonb_build_object('label', '30–100万', 'count', private.released_count(count(*) filter (where c.savings_cny >= 300000 and c.savings_cny < 1000000))),
        jsonb_build_object('label', '100万以上', 'count', private.released_count(count(*) filter (where c.savings_cny >= 1000000)))
      ),
      'retirement', jsonb_build_array(
        jsonb_build_object('label', '10年以内', 'count', private.released_count(count(*) filter (where c.retirement_years_remaining <= 10))),
        jsonb_build_object('label', '11–20年', 'count', private.released_count(count(*) filter (where c.retirement_years_remaining >= 11 and c.retirement_years_remaining <= 20))),
        jsonb_build_object('label', '21–30年', 'count', private.released_count(count(*) filter (where c.retirement_years_remaining >= 21 and c.retirement_years_remaining <= 30))),
        jsonb_build_object('label', '31–40年', 'count', private.released_count(count(*) filter (where c.retirement_years_remaining >= 31 and c.retirement_years_remaining <= 40))),
        jsonb_build_object('label', '40年以上', 'count', private.released_count(count(*) filter (where c.retirement_years_remaining >= 41)))
      )
    ),
    'matrices', jsonb_build_object(
      'workValue', jsonb_build_array(
        jsonb_build_object('label', '钱多事少', 'count', private.released_count(count(*) filter (where
          c.monthly_salary_cny >= (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
          and c.daily_work_minutes <= (select percentile_cont(0.5) within group (order by daily_work_minutes) from visible)
        ))),
        jsonb_build_object('label', '钱多事多', 'count', private.released_count(count(*) filter (where
          c.monthly_salary_cny >= (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
          and c.daily_work_minutes > (select percentile_cont(0.5) within group (order by daily_work_minutes) from visible)
        ))),
        jsonb_build_object('label', '钱少事少', 'count', private.released_count(count(*) filter (where
          c.monthly_salary_cny < (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
          and c.daily_work_minutes <= (select percentile_cont(0.5) within group (order by daily_work_minutes) from visible)
        ))),
        jsonb_build_object('label', '钱少事多', 'count', private.released_count(count(*) filter (where
          c.monthly_salary_cny < (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
          and c.daily_work_minutes > (select percentile_cont(0.5) within group (order by daily_work_minutes) from visible)
        )))
      ),
      'chillIndex', jsonb_build_array(
        jsonb_build_object('label', '摸鱼仙人', 'count', private.released_count(count(*) filter (where
          c.savings_cny is not null and c.retirement_years_remaining > 0
          and c.savings_cny / (c.retirement_years_remaining * 365.2425) >= (
            select percentile_cont(0.5) within group (order by savings_cny / (retirement_years_remaining * 365.2425))
            from visible where savings_cny is not null and retirement_years_remaining > 0
          )
          and c.monthly_salary_cny < (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
        ))),
        jsonb_build_object('label', '隐形富豪', 'count', private.released_count(count(*) filter (where
          c.savings_cny is not null and c.retirement_years_remaining > 0
          and c.savings_cny / (c.retirement_years_remaining * 365.2425) >= (
            select percentile_cont(0.5) within group (order by savings_cny / (retirement_years_remaining * 365.2425))
            from visible where savings_cny is not null and retirement_years_remaining > 0
          )
          and c.monthly_salary_cny >= (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
        ))),
        jsonb_build_object('label', '生存副本', 'count', private.released_count(count(*) filter (where
          c.savings_cny is not null and c.retirement_years_remaining > 0
          and c.savings_cny / (c.retirement_years_remaining * 365.2425) < (
            select percentile_cont(0.5) within group (order by savings_cny / (retirement_years_remaining * 365.2425))
            from visible where savings_cny is not null and retirement_years_remaining > 0
          )
          and c.monthly_salary_cny < (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
        ))),
        jsonb_build_object('label', '高薪长跑', 'count', private.released_count(count(*) filter (where
          c.savings_cny is not null and c.retirement_years_remaining > 0
          and c.savings_cny / (c.retirement_years_remaining * 365.2425) < (
            select percentile_cont(0.5) within group (order by savings_cny / (retirement_years_remaining * 365.2425))
            from visible where savings_cny is not null and retirement_years_remaining > 0
          )
          and c.monthly_salary_cny >= (select percentile_cont(0.5) within group (order by monthly_salary_cny) from visible)
        )))
      )
    )
  )
  from visible as c;
$$;

create or replace function public.get_public_messages(p_limit integer default 9)
returns table(kind text, text text)
language sql
stable
security definer
set search_path = ''
as $$
  select m.kind, m.body
  from private.public_messages as m
  where m.status = 'approved'
  order by md5(m.id::text || current_date::text)
  limit greatest(1, least(coalesce(p_limit, 9), 24));
$$;

revoke execute on function public.submit_public_data(integer, integer, integer, bigint, integer) from public, authenticated;
revoke execute on function public.submit_public_message(text, text) from public, authenticated;
revoke execute on function public.get_public_dashboard() from public, authenticated;
revoke execute on function public.get_public_messages(integer) from public, authenticated;

grant execute on function public.submit_public_data(integer, integer, integer, bigint, integer) to anon;
grant execute on function public.submit_public_message(text, text) to anon;
grant execute on function public.get_public_dashboard() to anon;
grant execute on function public.get_public_messages(integer) to anon;
