# Project documentation

This folder is the durable home for product requirements, technical choices,
implementation planning, and local hosting instructions. Update these
documents as decisions are made so the project does not depend on chat history.

## Stepped path to the web app

The [implementation roadmap](implementation/implementation-plan.md) describes
the phases and the check to complete before moving to the next one:

1. Define the intended users, problem, and meaning of “locally hosted.”
2. Agree on the first useful version and functional acceptance criteria in the
   [functional design](product/functional-design.md).
3. Sketch the key screens and journeys, including import, empty, and error
   states in the [experience design](product/experience-design.md).
4. Choose the architecture, stack, and sensitive-data controls in the
   [technical design](technical/technical-design.md).
5. Build in small increments, starting with the app foundation, then data
   import and dashboards.
6. Verify behavior and safety, then document and test the build, installation,
   activation, update, and recovery steps in the
   [local development and hosting guide](operations/local-development.md).

Keep the [decision log](decisions/decision-log.md) up to date throughout. Treat
the roadmap as a proposed sequence: do not start an implementation phase until
its entry conditions are met.

## Working agreements

- Treat unanswered prompts as open questions, not requirements or decisions.
- Keep product behavior in the functional design and implementation choices in
  the technical design.
- Record a decision in the decision log when it is agreed; link to the relevant
  design section.
- Never put passwords, API keys, real bank data, or other secrets in these
  documents. Document configuration names and safe setup steps instead.

## Current project context

The project is a single-user, private-home-network app for importing bank
account data, categorizing transactions, and reviewing finances. Hosting
constraints and the Triodos, ING, and VDK CSV schemas are documented; parser
implementation and deployment details remain for the technical and
implementation phases. Optional starter categories, conservative rules, and
their JSON import format are defined in the
[starter-set specification](product/starter-sets.md).
