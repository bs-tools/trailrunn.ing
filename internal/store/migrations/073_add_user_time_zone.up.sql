-- The time zone the user lives in, an IANA name like "America/Vancouver",
-- empty until it's set, which the web app does from the browser's. The
-- local times of their workouts are in it, and it's the zone of their
-- activities without a GPS track, which Strava otherwise guesses from the
-- watch's UTC offset.
alter table users add column time_zone text not null default '';
