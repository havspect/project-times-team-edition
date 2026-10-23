# Project Times

Team time tracking. People log intervals of work against projects, team leads read the hours logged on their projects, admins run the workspace.

## Language

**Workspace**:
One installation of the app, belonging to one company. Holds every user, team and project.
_Avoid_: tenant, organisation, org, account

**User**:
A person who can log in and log hours.
_Avoid_: account, employee, member (see **Member**)

**Admin**:
A user who may manage users, teams, projects, and restore deleted data.
_Avoid_: owner, superuser

**Team**:
A named group of users that owns projects and has one or more leads. A user may belong to several teams.
_Avoid_: group, department, squad

**Lead**:
A membership role on a team. Leads create and maintain that team's projects and read the hours logged on them.
_Avoid_: manager, owner, team lead (as a noun for the person; "lead" is fine)

**Member**:
A membership role with no extra rights: log own hours, read own totals.
_Avoid_: employee, developer, user

**Membership**:
The link between a user and a team, carrying the role of lead or member.
_Avoid_: seat, assignment

**Initial password**:
The password an admin sets for a new or reset user, which the user must replace at first login.
_Avoid_: temporary password, one-time password, OTP

**Deactivated user**:
A user who can no longer log in; their hours stay in reports.
_Avoid_: deleted user, disabled user

**Project**:
The unit of work that hours are logged against. Belongs to exactly one team and is visible to everyone in the workspace.
_Avoid_: task, ticket, job, client, engagement

**Entry**:
One user's continuous stretch of work on one project, with a start, an end and an optional note.
_Avoid_: record, timesheet, booking, time log

**Running entry**:
An entry with a start but no end yet. A user has at most one.
_Avoid_: timer, active entry, open record

**Log**:
To record an entry against a project.
_Avoid_: book, track, record (as a verb)

**Timeline**:
The personal, horizontally scrollable view of one user's entries over time, where logging happens.
_Avoid_: canvas, calendar, chart, board

**Note**:
Free text on an entry describing what was done.
_Avoid_: description, comment, remark

**Week**:
An ISO week: Monday to Sunday, labelled like `2026-W43`.
_Avoid_: calendar week, pay week

**Report**:
Totals of logged time, grouped by person, project and day or week.
_Avoid_: dashboard, stats, analytics

**Timezone**:
A user's own IANA zone. Decides which day and week that user's hours fall into.
_Avoid_: locale, region

**Archived project**:
A project hidden from project pickers whose logged hours still count in reports. Only an admin restores it.
_Avoid_: deleted project, inactive project

**Deleted entry**:
An entry removed from all lists and totals but kept in storage. Only an admin restores it.
_Avoid_: archived entry, voided entry
