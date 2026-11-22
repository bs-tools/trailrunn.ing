-- The running and touring gear each user has, as the names of plans.Gear,
-- and the apparent temperature from which they run in shorts, null for the
-- default. Workouts suggest what to wear and carry from it. Users without a
-- row haven't said what they have, and are suggested any gear.
create table gear_profiles (
  user_id uuid primary key references users(id) on delete cascade,
  gear text[] not null,
  shorts_min_apparent_celsius double precision,
  update_time timestamptz not null default current_timestamp
);
