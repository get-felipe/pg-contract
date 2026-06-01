drop view if exists customer_directory;
drop table if exists directory_customers;

create table directory_customers (
  id uuid primary key,
  email text not null,
  active boolean not null
);

create view customer_directory as
select id, active
from directory_customers;
