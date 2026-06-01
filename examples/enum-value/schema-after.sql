drop table if exists enum_orders;
drop type if exists enum_order_status;

create type enum_order_status as enum ('pending', 'paid', 'cancelled');

create table enum_orders (
  id uuid primary key,
  status enum_order_status not null
);
