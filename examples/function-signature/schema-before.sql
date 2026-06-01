drop function if exists format_customer_email(text);
drop function if exists format_customer_email(text, boolean);

create function format_customer_email(input text)
returns text
language sql
immutable
as $$
  select lower(input);
$$;
