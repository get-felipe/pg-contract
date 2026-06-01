-- name: billing.list_invoices
select id, total_cents
from invoices
where customer_id = $1;

