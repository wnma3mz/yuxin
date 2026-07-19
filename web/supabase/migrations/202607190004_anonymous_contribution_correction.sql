-- Allow a browser to correct its own anonymous numeric contribution without an account.
-- The raw credential stays in browser storage. The browser sends a SHA-256
-- verifier and the database stores a second SHA-256 digest of that verifier.

create schema if not exists extensions;
create extension if not exists pgcrypto with schema extensions;

alter table private.public_contributions
  add column if not exists edit_credential_hash bytea;

create unique index if not exists public_contributions_edit_credential_hash_key
  on private.public_contributions (edit_credential_hash)
  where edit_credential_hash is not null;

comment on column private.public_contributions.edit_credential_hash is
  'SHA-256 digest of a browser-derived correction verifier. Never exposed by public RPCs.';

comment on table private.public_contributions is
  'Coarse anonymous values. No account, profile, IP, user agent, exact timestamp, or raw correction credential.';

revoke execute on function public.submit_public_data(integer, integer, integer, bigint, integer)
  from public, anon, authenticated;
drop function public.submit_public_data(integer, integer, integer, bigint, integer);

create function public.submit_public_data(
  p_monthly_salary_cny integer,
  p_daily_work_minutes integer,
  p_workdays_per_week integer,
  p_savings_cny bigint,
  p_retirement_years_remaining integer,
  p_edit_credential_hash text default null
)
returns boolean
language plpgsql
security definer
set search_path = ''
as $$
declare
  normalized_salary integer;
  normalized_minutes smallint;
  normalized_savings bigint;
  credential_hash bytea;
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
  if p_edit_credential_hash is not null and p_edit_credential_hash !~ '^[0-9a-f]{64}$' then
    raise exception 'edit credential is invalid' using errcode = '22023';
  end if;

  normalized_salary := (round(p_monthly_salary_cny::numeric / 100) * 100)::integer;
  normalized_minutes := (round(p_daily_work_minutes::numeric / 30) * 30)::smallint;
  normalized_savings := case
    when p_savings_cny is null then null
    else (round(p_savings_cny::numeric / 1000) * 1000)::bigint
  end;

  if p_edit_credential_hash is null then
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
    return false;
  end if;

  credential_hash := extensions.digest(p_edit_credential_hash, 'sha256');
  update private.public_contributions
  set monthly_salary_cny = normalized_salary,
      daily_work_minutes = normalized_minutes,
      workdays_per_week = p_workdays_per_week,
      savings_cny = normalized_savings,
      retirement_years_remaining = p_retirement_years_remaining
  where edit_credential_hash = credential_hash;
  if found then
    return true;
  end if;

  begin
    insert into private.public_contributions (
      monthly_salary_cny,
      daily_work_minutes,
      workdays_per_week,
      savings_cny,
      retirement_years_remaining,
      edit_credential_hash
    ) values (
      normalized_salary,
      normalized_minutes,
      p_workdays_per_week,
      normalized_savings,
      p_retirement_years_remaining,
      credential_hash
    );
    return false;
  exception when unique_violation then
    update private.public_contributions
    set monthly_salary_cny = normalized_salary,
        daily_work_minutes = normalized_minutes,
        workdays_per_week = p_workdays_per_week,
        savings_cny = normalized_savings,
        retirement_years_remaining = p_retirement_years_remaining
    where edit_credential_hash = credential_hash;
    return true;
  end;
end;
$$;

revoke execute on function public.submit_public_data(integer, integer, integer, bigint, integer, text)
  from public, authenticated;
grant execute on function public.submit_public_data(integer, integer, integer, bigint, integer, text)
  to anon;
