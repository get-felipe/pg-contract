drop table if exists result_shape_customers;

create table result_shape_customers (
  id uuid primary key,
  email varchar(320) not null
);
