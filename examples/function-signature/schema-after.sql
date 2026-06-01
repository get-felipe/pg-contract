drop function if exists format_customer_email(text);
drop function if exists format_customer_email(text, boolean);

create function format_customer_email(input text, trim_spaces boolean)
returns text
language sql
immutable
as $$
  select case
    when trim_spaces then lower(btrim(input))
    else lower(input)
  end;
$$;
