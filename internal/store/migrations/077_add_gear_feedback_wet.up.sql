-- Whether it was wet during the workout the gear feedback is on: feeling
-- cold in the rain says nothing about dry days, so wet and dry workouts'
-- feedback adjusts the gear of wet and dry workouts apart.
alter table gear_feedback add column wet boolean not null default false;
