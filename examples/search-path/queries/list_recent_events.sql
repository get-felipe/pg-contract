-- name: events.list_recent
select id, occurred_at
from events
order by occurred_at desc;
