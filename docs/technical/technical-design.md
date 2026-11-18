# Technical design

**Status:** Not started — no technology choices have been made  
**Last updated:** YYYY-MM-DD

Complete this document after agreeing on the functional design. Record the
reason for significant choices in the [decision log](../decisions/decision-log.md).

## Constraints and quality attributes

- Functional design reference: [Functional design](../product/functional-design.md)
- Required operating systems and hardware: To decide
- Expected data volume and number of users: To decide
- Availability, performance, and maintainability needs: To decide

## Architecture

- Application shape and components: To decide
- Frontend and backend responsibilities: To decide
- Communication boundaries: To decide
- Diagram or reference: To add when the architecture is agreed

## Technology choices

| Concern | Choice | Rationale | Status |
| --- | --- | --- | --- |
| Language and framework | To decide | To document | Open |
| Database and data access | To decide | To document | Open |
| Build and package tooling | To decide | To document | Open |
| Test tooling | To decide | To document | Open |

## Data and imports

- Supported import sources and formats: To decide
- Validation, duplicate detection, and error handling: To decide
- Data model and schema migration approach: To decide
- Import provenance and audit needs: To decide
- Backup, restore, export, and deletion behavior: To decide

## Security and privacy

Bank and transaction data is sensitive. Define and review the controls before
implementing real-data support.

- Where data is stored and who can access it: To decide
- Authentication and session behavior, if needed: To decide
- Encryption needs and key management: To decide
- Handling of logs, diagnostics, and sample data: To decide
- Dependency updates and security testing: To decide

Do not store real credentials, API keys, or bank data in this repository.

## Verification

- Unit, integration, and end-to-end testing approach: To decide
- Data import and migration test strategy: To decide
- Accessibility and browser verification: To decide
- Build, lint, and type-check commands: To document after tooling is selected

## Open technical questions

- Which hosting and storage model best meets the agreed meaning of “local”?
- Should imports use bank-provider integrations, uploaded files, or both?
- What backup and recovery experience is required?
- To decide
