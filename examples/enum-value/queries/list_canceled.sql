-- name: orders.list_canceled
select id
from enum_orders
where status = 'canceled';
