# Decision log

Record significant product and engineering decisions here so they remain
available outside chat. Add a new entry rather than silently changing a
previous decision. If a decision is superseded, link the replacement.

## Decisions

### DEC-001: Agree first-version functional baseline

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** Phase 2 defined the first-version user needs, functional
  requirements, and observable acceptance criteria.
- **Decision:** Use the requirements and acceptance criteria in the functional
  design as the agreed baseline. Keep bank-format validation dependent on
  sanitized samples; defer technical choices and explicitly deferred behavior
  to the relevant later design phases.
- **Alternatives considered:** Keep the requirements and acceptance criteria
  provisional and continue Phase 2.
- **Consequences:** Phase 2 is complete and screen/flow design can proceed.
  Deferred questions remain open for their designated phases.
- **Related documents:** [Functional design](../product/functional-design.md);
  [implementation plan](../implementation/implementation-plan.md).

### DEC-002: Agree first-version screen and dashboard proposal

- **Status:** Accepted
- **Date:** 2026-11-18
- **Context:** Phase 3 reviewed the proposed navigation, dashboard, import
  history, and repeated-payee categorization details.
- **Decision:** Use the six proposed primary destinations. Keep recent import
  jobs on the Import landing screen. Use the four proposed dashboard charts,
  with category filtering applied to every chart. Show repeated-payee groups
  with relevant summary context and expandable transaction-level details.
- **Alternatives considered:** Limit category filtering to category charts,
  provide a separate import-history destination, or reduce the detail shown
  for repeated-payee groups.
- **Consequences:** Phase 3 is complete. Detailed visual styling and technical
  behavior remain for later phases.
- **Related documents:** [Experience design](../product/experience-design.md);
  [functional design](../product/functional-design.md).

## Entry template

Copy this section for each decision:

### DEC-001: Short title

- **Status:** Proposed / Accepted / Superseded
- **Date:** YYYY-MM-DD
- **Context:** What question or constraint prompted this decision?
- **Decision:** What was chosen?
- **Alternatives considered:** What other reasonable options were considered?
- **Consequences:** What trade-offs, follow-up work, or risks result?
- **Related documents:** Links to relevant design sections
