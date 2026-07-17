-- Add a privacy-thresholded public "what if everyone stopped working" metric.
-- Run this after 202607170001_public_dashboard.sql on existing projects.

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

revoke execute on function public.get_public_dashboard() from public, authenticated;
grant execute on function public.get_public_dashboard() to anon;
