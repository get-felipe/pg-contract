drop table if exists customers;

create table customers (
  id uuid primary key,
  name text not null,
  email text not null
);
