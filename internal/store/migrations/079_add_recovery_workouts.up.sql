-- Recovery workouts: electrical muscle stimulation, massage, compression,
-- heat or cold, done to recover rather than to train.
alter type workout_type add value 'recovery';

-- What a recovery workout's items are, and whether the user did it, as
-- JSON, as it's only read and written with the workout. Null for other
-- workouts.
alter table workouts add column recovery jsonb;
