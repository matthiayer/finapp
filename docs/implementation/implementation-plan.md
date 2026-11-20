# Implementation plan

**Status:** Phases 1–7 complete; Phase 8 is in progress
**Last updated:** 2026-11-20

The goal is a useful, locally hosted web app for importing bank account data
and creating dashboards. Follow these phases in order: decide what to build,
define how it should work, choose how to build it, then implement and host it.
Keep each phase small and review its exit check before proceeding. The sequence
is a working proposal; remaining deployment-specific values and implementation
details will be resolved before and during implementation.

## Phases

| Phase | Work and deliverables | Exit check | Status |
| --- | --- | --- | --- |
| 1. Agree on the outcome | Identify intended users, the problem to solve, what “locally hosted” means, and the smallest useful first version. Record scope and non-goals in the [functional design](../product/functional-design.md). | Initial user, problem, first-version direction, and private-network hosting boundary are recorded. | Complete — initial direction |
| 2. Define behavior | Detail the first-use, data-import, and dashboard journeys. Define requirements and observable acceptance criteria, including errors, corrections, and data deletion. | The first-version requirements and acceptance criteria are agreed; open questions that could change the scope are resolved or explicitly deferred. | Complete |
| 3. Design the experience | Sketch the main screens and navigation. Include loading, empty, success, and error states for importing and viewing data. Capture screen and flow proposals in the [experience design](../product/experience-design.md). | The key journeys can be reviewed end to end without relying on unstated behavior. | Complete |
| 4. Define the technical approach | Choose the architecture and stack. Decide how account data is imported, stored, protected, backed up, and deleted. Design an offline anonymization approach for any real bank-export samples needed to confirm parser behavior. Document testing and migration approaches in the [technical design](../technical/technical-design.md), and record significant choices in the [decision log](../decisions/decision-log.md). | The technical design supports the agreed requirements; sensitive-data handling, bank-sample anonymization, and the local hosting boundary have been reviewed. | Complete |
| 5. Build the foundation | Create the app skeleton, local configuration pattern, data-storage foundation, and build/test checks. Define how source is built and packaged into a GitHub release. Use the existing browser-only local CSV sanitizer if real export samples are needed. | A developer can build and run the app locally; automated checks run; the release package can be produced without real financial data or secrets. | Complete — local build, checks, HTTP smoke test, and Linux AMD64 package verified |
| 6. Implement data import | Implement the agreed import path, validation, duplicate handling, error recovery, and user feedback. Use synthetic bank-shaped fixtures based on the documented Triodos, ING, and VDK layouts; use sanitized samples only when a format detail needs confirmation. | Agreed import acceptance criteria pass, including representative invalid and duplicate data cases using the agreed exact-match criteria; each documented bank CSV variant passes parser tests. | Complete — supported synthetic formats and import acceptance checks pass; pending-row detection is explicitly deferred |
| 7. Implement dashboards | Build the agreed dashboard views and interactions on top of imported data, including empty and no-data states. | Agreed dashboard acceptance criteria pass using test data, and the core journey works from import through viewing results. | Complete — preset/custom-range filters, empty state, four charts, category filtering, and the unfiltered actual-balance chart verified by Go and browser tests |
| 8. Harden and host locally | Verify privacy controls, accessibility, error handling, backups, and recovery. Complete and test the step-by-step [installation, build, activation, and hosting guide](../operations/local-development.md), including upgrade and recovery instructions. | A new installation can be built or installed from a GitHub release, configured, activated as a service, verified, updated, and recovered using only the documented steps; access boundaries are clear and tested. | In progress — installation and recovery procedures documented; target LXC verification pending |

## Implementation increments

After Phases 1–4, split the implementation into small changes that can be
tested and reviewed independently. Track each increment here:

| Increment | User outcome | Scope | Dependencies | Verification | Status |
| --- | --- | --- | --- | --- | --- |
| App foundation | Developer can build, configure, run, and package the app | Go server, environment configuration, embedded assets, SQLite/Goose startup, initial schema, systemd defaults, CI/release workflows | Phases 1–4 | `make check`; local HTTP/health smoke test; Linux AMD64 tar package contents inspected | Complete |
| Bank parsers and transactional import service | Bank CSV rows can be validated and imported with explicit duplicate and rate choices | Triodos/ING/VDK parsers, 1 MB per-file check, in-memory preview, row errors, duplicate detection, EUR conversion, atomic commit and provenance | App foundation | `go test ./internal/bankcsv ./internal/importflow ./internal/storage` | Complete for the documented synthetic formats |
| Import web workflow | User can upload, review, confirm, and inspect jobs; choose starter categories and rules | Multi-file form, account confirmation, duplicate/rate choices, active-rule category preview/totals, explicit conflict choices, starter category/rule selection with preview and transactional application to existing matches, result, recent-job list, transaction view | Bank parsers and import service | Web integration tests from upload through result and starter selection; rule match/conflict and category-total tests; browser acceptance | Complete — pending-row handling is deferred because supported layouts have no explicit marker |
| Dashboard | User can inspect balances and cash-flow trends | Dashboard aggregates, time/category filters, balances, four bundled charts, empty/no-data states | Imported transactions | Dashboard unit tests, `go test ./internal/web`, Chrome/Playwright acceptance | Complete — custom range interaction and balance-chart category-filter exception covered |

## Definition of done

Each increment should:

- Meet its linked acceptance criteria and include appropriate tests.
- Pass the agreed build, test, lint, or type-check commands.
- Update relevant documentation and setup instructions.
- Avoid committing real financial data or secrets.

## Delivery notes

- Branching: Use `main` only; no feature branches.
- Release numbering: Sequential labels `v1`, `v2`, `v3`, and onward.
- Additional release-specific review or verification: None required; normal
  implementation tests and checks still apply.
- Release rollback: Not required.
- Known risks and mitigations:
  - No app sign-in: any device allowed by the home network can access
    financial data. Keep the existing home-only network boundary, add no
    external route, and verify this boundary during deployment.
  - Backups contain sensitive financial data and are not encrypted by the app.
    Warn before backup and keep backup files outside the release and source
    repository.
  - Bank exports may vary from the documented layouts. Use synthetic fixtures
    by default, validate with the local sanitizer when needed, and reject
    invalid import jobs without partially saving them.
  - Import preview applies persisted enabled rules and reports category totals.
    The user-facing starter category/rule selection, review, and confirmation
    flow is implemented. Pending-row filtering is deferred for v1 because
    supported layouts provide no explicit status marker. Phases 6 and 7 are
    verified; Phase 8 still requires testing on the target LXC.
