# GORM AutoMigrate owns the schema, on a pure-Go SQLite driver

Persistence is GORM over `glebarez/sqlite` (modernc, no cgo), so the binary still cross-compiles and ships without a C toolchain. The GORM models are the schema: `AutoMigrate` runs them at boot, and there are no hand-written migration files. One raw statement follows it in the same boot path to create the partial unique index that enforces a single running entry, because struct tags cannot express `WHERE end_at IS NULL`.

Why: AutoMigrate keeps the schema in one place and removes a file that every contributor forgets to write. The default cgo SQLite driver would have quietly traded the single-binary install for it.

Consequence: AutoMigrate only adds. Dropping or renaming a column, changing a column type on SQLite, or moving existing data needs manual SQL against the production database; that is the upgrade path. A database also has no recorded schema version, so support starts with `tstamps version` plus the model set. Column additions land with a zero default, so a new field is not done until its model and its store test agree.
