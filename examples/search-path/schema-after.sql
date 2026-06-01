drop schema if exists contract_events cascade;
create schema contract_events;

create table contract_events.events (
  id uuid primary key,
  occurred_at timestamptz not null
);

set search_path = public;
