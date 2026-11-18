# Implementation plan

**Status:** Proposed roadmap — details to be agreed as the designs progress
**Last updated:** 2026-11-16

The goal is a useful, locally hosted web app for importing bank account data
and creating dashboards. Follow these phases in order: decide what to build,
define how it should work, choose how to build it, then implement and host it.
Keep each phase small and review its exit check before proceeding. The sequence
is a working proposal; technical choices and exact scope remain open.

## Phases

| Phase | Work and deliverables | Exit check | Status |
| --- | --- | --- | --- |
| 1. Agree on the outcome | Identify intended users, the problem to solve, what “locally hosted” means, and the smallest useful first version. Record scope and non-goals in the [functional design](../product/functional-design.md). | Initial user, problem, first-version direction, and private-network hosting boundary are recorded. | Complete — initial direction |
| 2. Define behavior | Detail the first-use, data-import, and dashboard journeys. Define requirements and observable acceptance criteria, including errors, corrections, and data deletion. | The first-version requirements and acceptance criteria are agreed; open questions that could change the scope are resolved or explicitly deferred. | In progress |
| 3. Design the experience | Sketch the main screens and navigation. Include loading, empty, success, and error states for importing and viewing data. Capture the agreed flows in the functional design. | The key journeys can be reviewed end to end without relying on unstated behavior. | Not started |
| 4. Define the technical approach | Choose the architecture and stack. Decide how account data is imported, stored, protected, backed up, and deleted. Document testing and migration approaches in the [technical design](../technical/technical-design.md), and record significant choices in the [decision log](../decisions/decision-log.md). | The technical design supports the agreed requirements; sensitive-data handling and the local hosting boundary have been reviewed. | Not started |
| 5. Build the foundation | Create the app skeleton, local configuration pattern, data-storage foundation, and build/test checks. Add setup instructions as decisions become concrete. | A developer can start the app locally; automated checks run; no real financial data or secrets are needed to verify the foundation. | Not started |
| 6. Implement data import | Implement the agreed import path, validation, duplicate handling, error recovery, and user feedback. Use synthetic or sanitized fixtures for development and tests. | Agreed import acceptance criteria pass, including representative invalid and duplicate data cases. | Not started |
| 7. Implement dashboards | Build the agreed dashboard views and interactions on top of imported data, including empty and no-data states. | Agreed dashboard acceptance criteria pass using test data, and the core journey works from import through viewing results. | Not started |
| 8. Harden and host locally | Verify privacy controls, accessibility, error handling, backups, and recovery. Document exact setup, run, stop, and data-location instructions in [local development and hosting](../operations/local-development.md). | The app can be set up and used locally from the documented steps; access boundaries and data recovery behavior are clear and tested. | Not started |

## Implementation increments

After Phases 1–4, split the implementation into small changes that can be
tested and reviewed independently. Track each increment here:

| Increment | User outcome | Scope | Dependencies | Verification | Status |
| --- | --- | --- | --- | --- | --- |
| To define after design | To define | To define | To define | Link acceptance criteria and checks | Not started |

## Definition of done

Each increment should:

- Meet its linked acceptance criteria and include appropriate tests.
- Pass the agreed build, test, lint, or type-check commands.
- Update relevant documentation and setup instructions.
- Avoid committing real financial data or secrets.

## Delivery notes

- Branching and review expectations: To decide
- Release and rollback approach: To decide
- Known risks and mitigations: To decide
