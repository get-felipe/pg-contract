-- name: directory.list_active_customers
select id, email
from customer_directory
where active;
