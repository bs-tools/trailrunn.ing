-- How the gear felt on a workout, for each part of the body: 'too_cold',
-- 'just_right' or 'too_warm', null if not rated. The user's feedback, in
-- the order of their workouts, adjusts the temperatures their gear is
-- suggested for.
create table gear_feedback (
  workout_id uuid primary key references workouts(id) on delete cascade,
  user_id uuid not null references users(id) on delete cascade,
  legs text,
  body text,
  hands text,
  head text,
  notes text not null default '',
  update_time timestamptz not null default current_timestamp
);

create index gear_feedback_user_id on gear_feedback (user_id);
