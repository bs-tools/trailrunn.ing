-- The heart rate range to hold on a run without structure, as JSON like the
-- targets of a structured run's steps: in beats per minute, relative to a
-- threshold, or a range of zones. Null for a run that has the default, and
-- for other workouts.
alter table workouts add column run_heart_rate jsonb;
