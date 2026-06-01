drop table if exists user_actions;
drop table if exists users;

create table users (
  id uuid primary key,
  name text not null,
  created_at timestamptz not null
);

create table user_actions (
  id uuid primary key,
  user_id uuid not null references users(id),
  created_at timestamptz not null
);
