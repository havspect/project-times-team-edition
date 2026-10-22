# Nothing is hard-deleted; projects archive, entries delete

Deleting a project archives it: it leaves the project pickers but its hours keep counting in reports. Deleting an entry removes it from lists and totals but keeps the row; only an admin restores it. Users are deactivated rather than removed, so their history stays readable.

Why: leads hand these numbers to clients and payroll. A finished project must not erase a quarter of hours, and a mis-click must not silently change a total that was already sent.

Consequence: every query that produces totals filters deleted entries explicitly, and every list of pickable projects filters archived ones. Both live in the store layer, not in handlers.
