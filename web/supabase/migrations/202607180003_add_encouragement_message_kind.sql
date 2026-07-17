-- Add the fourth moderated anonymous-message category.
-- Run this after 202607180002_public_lay_flat_metric.sql on existing projects.

alter table private.public_messages
  drop constraint if exists public_messages_kind_check;

alter table private.public_messages
  add constraint public_messages_kind_check
  check (kind in ('advice', 'rant', 'wish', 'encourage'));

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

revoke execute on function public.submit_public_message(text, text) from public, authenticated;
grant execute on function public.submit_public_message(text, text) to anon;
