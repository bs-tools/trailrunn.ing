-- Where the user's next lactate test expects their aerobic threshold, which
-- the test has closer stages around. Set by the user, and from each test
-- that finds a threshold. Null if they cleared it, for a test with even
-- steps; without a row, it's the threshold of their latest test.
create table lactate_profiles (
  user_id uuid primary key references users(id) on delete cascade,
  aerobic_threshold_heart_rate_bpm integer,
  update_time timestamptz not null default current_timestamp
);
