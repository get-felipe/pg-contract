-- name: customers.find_customer
select id, name, email
from customers
where id = $1;
