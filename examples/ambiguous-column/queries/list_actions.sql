-- name: audit.list_actions
select ua.id, u.name
from user_actions ua
join users u on u.id = ua.user_id
order by created_at;

