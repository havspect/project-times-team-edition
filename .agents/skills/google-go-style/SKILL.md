---
name: google-go-style
description: Check Go code against the Google Go style guide (guide, decisions, best practices). Use when asked to review Go style, audit compliance with Google's Go style, or check a Go change for style issues.
---

# Google Go Style Check

Report style violations. Do not edit code unless the user explicitly asks for fixes after the report.

Sources ([google/styleguide `go`](https://github.com/google/styleguide/tree/gh-pages/go), branch `gh-pages`), in precedence order. Local copies live next to this file; read those, don't download them again:

1. [Guide](references/guide.md) — normative, canonical
2. [Decisions](references/decisions.md) — normative
3. [Best practices](references/best-practices.md) — advisory

When a rule is ambiguous, `grep` the local file for that heading before reporting it. Do not report it from memory if the file can be checked. Only re-download when the user asks for fresh upstream text:

```
cd .agents/skills/google-go-style && for f in guide decisions best-practices; do curl -fsSL "https://raw.githubusercontent.com/google/styleguide/gh-pages/go/$f.md" -o "references/$f.md"; done
```

Downloaded from `gh-pages` on 2026-10-22.

## Scope

- User named files or paths: check those.
- Uncommitted Go changes: `git diff --name-only -- '*.go'` plus `git diff --cached`.
- Otherwise: every `*.go` file, skipping `vendor/` and generated files.
- Reviewing a diff: report the new lines and the call sites they reach. Do not re-litigate unrelated old code.
- Generated code (`// Code generated ... DO NOT EDIT.`) is out of scope except formatting.

## Automated pass

Run these first and report failures before the manual review:

```
gofmt -l .
go vet ./...
go build ./...
```

`go test ./...` only when the user asks for correctness too. `gofmt -s -l .` is optional. Do not install linters: staticcheck is not part of the style guide.

## Checklist

### Naming — [decisions#naming](references/decisions.md#naming)

- N1 Package names: lowercase, no underscores, short, informative. `util`, `common`, `helper`, `model` are findings.
- N2 MixedCaps everywhere; flag underscores in identifiers (tests excepted).
- N3 Initialisms keep one case: `ID`, `URL`, `DB`, `API`, not `Id`, `Url`.
- N4 Receiver: 1–2 letters, abbreviation of the type, same name on every method, never `this` or `self`.
- N5 No `Get` prefix unless the concept is “get” (as in HTTP GET).
- N6 No repetition: `widget.NewWidget` → `widget.New`; methods don’t repeat the receiver; variables don’t repeat the type (`userSlice`, `nameString`).
- N7 Name length grows with scope. Single letters only for tiny scopes and familiar cases (`i`, `r`, `w`).
- N8 Constants are MixedCaps (`MaxPacketSize`), no `k` prefix, named for their role not their value.

### Comments and docs — [decisions#commentary](references/decisions.md#commentary)

- D1 Every exported top-level name has a doc comment starting with the name, as a full sentence (`// Config holds ...`).
- D2 Exactly one package comment per package, immediately above `package`: `// Package config ...` or `// Command times ...`.
- D3 Comments explain why, not what. Delete comments that restate the code.
- D4 Doc comments are capitalized and punctuated; end-of-line field comments may be fragments.
- D5 Document only non-obvious parameters, context behavior, and concurrency. Don’t restate that a cancelled context returns `ctx.Err()`.
- D6 Don’t wrap lines in a way that creates indentation confusion; don’t split a long string or URL.

### Imports — [decisions#imports](references/decisions.md#imports)

- I1 Groups: stdlib, then third-party/project, then side-effect imports, separated by blank lines.
- I2 No dot imports.
- I3 Blank imports only in `main` or tests (or `embed`).
- I4 Renames are lowercase, without underscores, and consistent. Rename only for collisions or uninformative package names.

### Errors — [decisions#errors](references/decisions.md#errors) · [best-practices#error-handling](references/best-practices.md#error-handling)

- E1 `error` is the last result.
- E2 Error strings lowercase, no trailing punctuation, no redundant context (`failed: %v` adds nothing).
- E3 Handle or return every error. `_` is allowed only with a comment saying why the call cannot fail.
- E4 Wrap with `%w` at the end (`...: %w`), except sentinel errors where the category goes first.
- E5 Indent error flow: handle and return; no `else` after a terminating `if`.
- E6 No in-band errors (`-1`, `""`, `nil`); return `(value, ok)` or `(value, error)`.
- E7 No `panic` in library code and no `recover` to hide bugs. `must*` helpers only for init-time invariants.

### Language and API — [decisions#language](references/decisions.md#language) · [guide#simplicity](references/guide.md#simplicity)

- L1 `context.Context` is the first parameter, never a struct field. Do not create custom context types.
- L2 Return concrete types, accept interfaces, define interfaces at the consumer. Question interfaces with one implementation.
- L3 No type aliases for new code; use defined types.
- L4 Prefer synchronous functions. If you start a goroutine, document its lifetime and how it ends.
- L5 `any`, not `interface{}`.
- L6 `%q` for quoted strings in errors and logs.
- L7 Prefer `var s []T` over `[]T{}` when empty; don’t distinguish nil from empty in APIs.
- L8 Struct literals: field names for external types, omit zero values, matching braces, no cuddled braces unless both sides are literals.
- L9 Don’t copy a value containing a `sync.Mutex`; use pointer or value receivers consistently for a type.
- L10 Don’t shadow standard package names (`url`, `path`, `errors`) with variables; use `:=` deliberately in inner scopes.

### Tests — [decisions#useful-test-failures](references/decisions.md#useful-test-failures) · [best-practices#tests](references/best-practices.md#tests)

- T1 `t.Errorf` when the test can continue; `t.Fatal` only for setup failure. Never `t.Fatal` in a goroutine.
- T2 Failure messages identify the function, input, got, and want: `Foo(%q) = %v, want %v`.
- T3 Compare whole structures, not field by field; use `cmp.Diff` or `errors.Is` where available.
- T4 No assertion libraries; plain `if` plus `t.Errorf`.
- T5 Table-driven tests with named subtests via `t.Run`; test structs use field names.
- T6 Test helpers call `t.Helper()`; test doubles follow `packagetest` plus `Stub` or behavior names.
- T7 Use real transports and implementations over hand-rolled fakes where practical.

### Structure and dependencies — [best-practices#util-packages](references/best-practices.md#util-packages) · [best-practices#global-state](references/best-practices.md#global-state)

- S1 No `util`/`common`/`helper` packages; name the package after what it does.
- S2 Least mechanism: core language, then stdlib, then existing dependency, then a new dependency.
- S3 Keep mutable package-level state out of libraries; inject dependencies (this repo’s `Clock` is the pattern).
- S4 `crypto/rand` for anything security-relevant; never `math/rand`.
- S5 No premature options or config knobs; a plain argument or struct field first.
- S6 String building: `+` for simple cases, `fmt.Sprintf` for formatting, `strings.Builder` in loops.
- S7 Flags live in `main` or the CLI package, use snake_case names, and don’t leak out of imported libraries.

## Report

One line per finding:

```
severity — file:line — rule — what and why — fix
```

Severity:

- **Blocking** — gofmt/vet/build failure, discarded error, panic in a library, security issue (`math/rand`, unvalidated input at a trust boundary).
- **Should fix** — naming, missing docs, error strings or wrapping, context placement, weak test messages.
- **Nit** — clarity and concision judgement calls, style the guide leaves open.

Close with per-section pass/fail and a verdict: `N blocking, N should-fix, N nits`. If clean, say so and list what you checked.

## Not applicable here

Do not report Bazel `BUILD` rules, proto import suffixes, gRPC status codes, `glog`/`log.Exit`, Google-internal flag wrappers, or `nogo`. Use the open-source equivalents already in the repo (stdlib `flag`, `log`, or `slog` if added).
