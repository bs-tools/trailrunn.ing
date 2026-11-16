-- name: GetLactateProfile :many
-- Gets the user's lactate profile: no rows if it was never set.
select * from lactate_profiles
where user_id = $1;

-- name: SetLactateProfile :one
insert into lactate_profiles (user_id, aerobic_threshold_heart_rate_bpm)
values ($1, $2)
on conflict (user_id) do update set
  aerobic_threshold_heart_rate_bpm = excluded.aerobic_threshold_heart_rate_bpm,
  update_time = now()
returning *;

-- name: SetLactateProfileFromTest :exec
-- Sets the user's expected aerobic threshold to the one a test found, if
-- it's their latest test: an older test that's edited doesn't say where the
-- threshold is now.
insert into lactate_profiles (user_id, aerobic_threshold_heart_rate_bpm)
select t.user_id, sqlc.arg(aerobic_threshold_heart_rate_bpm)::integer
from lactate_tests t
where t.id = sqlc.arg(test_id) and not exists (
  select 1 from lactate_tests later
  where later.user_id = t.user_id and later.id <> t.id
    and (later.date, later.create_time) > (t.date, t.create_time)
)
on conflict (user_id) do update set
  aerobic_threshold_heart_rate_bpm = excluded.aerobic_threshold_heart_rate_bpm,
  update_time = now();
