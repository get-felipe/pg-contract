-- name: search.count_tags
select array_length($1, 1) as tag_count;

