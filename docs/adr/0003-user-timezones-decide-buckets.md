# A user's own timezone decides their day and week

_Superseded by [ADR-0007](./0007-entry-timezones-decide-buckets.md): the entry's timezone now decides, not the user's._

Timestamps are stored in UTC. Every user has an IANA timezone, and hours are bucketed into days and ISO weeks in the timezone of the person whose hours they are — not the viewer's. A lead in Berlin and a developer in Auckland see the same weekly total for that developer.

Why: one canonical number per person-week. Bucketing by viewer would make the same report disagree between two leads, and somebody would eventually reconcile the two numbers by hand.

Consequence: a total can look wrong against the viewer's own calendar (Auckland's Monday morning is Berlin's Sunday). The API returns each row's timezone so the UI can say whose day it is. Reports pad the UTC query window by 14 hours and split entries at local midnight in Go rather than in SQL.
