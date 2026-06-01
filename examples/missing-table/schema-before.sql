drop table if exists invoices;

create table invoices (
  id uuid primary key,
  customer_id uuid not null,
  total_cents bigint not null
);
