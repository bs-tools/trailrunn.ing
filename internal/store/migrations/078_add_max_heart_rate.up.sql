-- The user's maximum heart rate, which heart rate zone 4 goes up to. Null
-- to estimate it from their activities.
alter table lactate_profiles add column max_heart_rate_bpm integer;
