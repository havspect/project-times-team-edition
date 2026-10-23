# An entry records the timezone it was logged in

Timestamps stay UTC, but every entry also stores the IANA zone in which it was logged. Days and ISO weeks are bucketed by that entry's zone, not by the user's profile zone. Clients send the zone they are running in (browser or machine), and the entry keeps it when the user later travels or changes their profile.

Why: a German user working from Australia should type 09:00, not 23:00, without changing their profile zone and re-bucketing months of history. Bucketing by the entry's zone keeps one number for the traveller and for a lead in another zone — the guarantee ADR-0003 protected — while the profile zone alone could not express where a particular day happened.

Considered options: the viewer's device zone (rejected: two leads would disagree, the failure ADR-0003 exists to prevent); the profile zone for everything (rejected: travel would either rewrite history or record a working day as its home-zone night); changing the profile zone while travelling (rejected: it moves every past entry). A single combined timestamp-and-zone column was rejected too: an offset is not a zone, and SQLite compares text, so a mixed-format column cannot answer "which name" or sort by instant.

Consequence: `timezone` is required on `POST /entries` and `POST /timer/start` (`422` without it) and accepted by `PATCH /entries/{id}` for corrections. Existing rows are backfilled from their owner's profile zone — a guess that cannot recover where they were actually logged. The edit panel shows an entry's times in its own zone, so retiming does not move the instant. Reports and the CSV detail carry each entry's zone. The user's profile zone no longer decides buckets; it remains only a default for undated range queries and for admin display.
