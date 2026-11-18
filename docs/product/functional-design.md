# Functional design

**Status:** Draft — Phase 1 captured; Phase 2 behavior being defined
**Last updated:** 2026-11-16

Describe what the app should help people do. Keep this document focused on
user needs and observable behavior; put technology choices in the
[technical design](../technical/technical-design.md).

## Problem and intended users

- **Problem to solve:** The user has accounts at multiple banks but lacks one
  place to understand balances, spending, and overall financial activity. They
  are concerned that unnoticed spending could make it difficult to save.
- **Initial intended user:** The user themselves.
- **Possible future users:** Their household; multi-user behavior is not yet in
  the first-version scope.
- **Where and how users expect to use the app:** Hosted on a private home
  network and accessed from devices on that network. The exact access and
  security behavior still needs technical definition.

## Goals and non-goals

### Goals

- Bring information from multiple bank accounts into one app. Manual import is
  acceptable for the first version if needed.
- Provide three distinct areas: importing information, viewing dashboards, and
  categorizing transactions. Categorization is a separate module within the
  app.
- Show an overview of finances, including balances and history, and make
  multi-year financial trends easier to understand.
- Help the user understand spending and income so activity that may affect
  saving is easier to notice.

### Non-goals

- Automatic outlier detection or alerts are not part of the first version.
- Recurring-payment or transaction-pattern analysis is not part of the first
  version; the focus is on financial trends over time.

## User journeys

For each journey, describe the starting point, the user's actions, the outcome,
and important error or recovery paths.

### First use and setup

- Initial user: the account owner. Setup details are to define.

### Importing or updating account data

- The first version should import files exported from banks.
- The target is to import up to five years of transaction history where the
  banks make it available.
- Before saving, the app should show a preview of the selected file and let the
  user confirm the import.
- If an import may contain transactions already imported, the app should show
  the possible duplicates and ask the user how to handle them rather than
  silently adding or discarding them.
- If a file contains both valid and invalid rows, the app should report the
  invalid rows and let the user import valid rows only.
- To decide: which banks and file formats should be supported?
- To decide: how should users review, correct, or undo an import?

#### Public bank-export research

Research checked on 2026-11-16. These public help pages do not confirm what is
available for the user's specific accounts; verify export options in the bank's
own service before finalizing format support.

The user can provide export files later to help verify real-world file
structure. Before sharing, use a small redacted or synthetic sample; do not
share account identifiers, names, balances, or real transaction details. Any
sample used for implementation or tests must be sanitized and kept out of
version control.

| Bank | Publicly documented information | Five-year availability |
| --- | --- | --- |
| ING Belgium | No public information found confirming transaction-export formats or history limits. | Unknown |
| Triodos Belgium | [Internet Banking export FAQ](https://www.triodos.be/nl/meest-gestelde-vragen/kan-ik-gegevens-vanuit-internet-banking-naar-mijn-boekhoudpakket-exporteren?id=31b679c364da) lists Excel, CSV, and CODA for exporting account data to accounting software. A [separate FAQ](https://www.triodos.be/nl/meest-gestelde-vragen/hoe-kan-ik-mijn-rekeninguittreksels-raadplegen?id=e6f9b40c28bd) documents PDF statements. | Not stated publicly |
| vdk bank | The [account movements FAQ](https://www.vdk.be/nl/veelgestelde-vragen/ik-wil-een-overzicht-van-de-bewegingen-op-mijn-rekeningen-waar-vind-ik-dat) says processed movements can be viewed up to ten years back. The public pages found do not confirm a structured transaction-export format. A separate [statement FAQ](https://www.vdk.be/nl/veelgestelde-vragen/hoelang-blijven-mijn-elektronische-rekeninguittreksels-beschikbaar) says electronic statements remain in history for two years. | Five years of movements can be viewed, but five years of downloadable transaction data is not confirmed |

### Creating and viewing dashboards

- The first version should show combined balances across accounts, income and
  spending over time, transaction history, and spending by group.
- Show each account's latest known balance with its date. The combined balance
  should use each account's latest known balance and make clear that those
  balances may be from different dates.
- If an import does not provide an account balance, ask the user to enter the
  current balance and record when it applies. Use that balance and the imported
  transactions to calculate historical balances for the covered period.
- If transaction history has a gap, continue with a best-effort calculation
  using the available transactions and do not block the rest of the app. Show a
  non-blocking error for the user to fix, and clearly mark affected historical
  balances as incomplete until the gap is resolved or explicitly ignored.
- For each gap issue, let the user choose to be reminded later (leaving it open
  as an action), upload additional files to close the gap, or explicitly ignore
  the gap. Ignoring it closes the action and removes the incomplete marker from
  affected balances.
- The default dashboard time window should be the rolling last 12 months, with
  the ability to change the selected time window.
- Show total incoming money and spending by group. Income may have groups such
  as Salary, so the user can see salary separately when they categorize it.
- The user defines saving as income minus spending.
- Exclude confirmed internal transfers from the income and spending totals used
  to calculate saving.
- Show multi-year trends in spending, saving, earning, and account balances as
  monthly totals.
- Internal transfers should be shown separately from income and spending.
- The app should suggest likely pairs of outgoing and incoming transactions
  as transfers when they are in opposite directions, have the same amount, are
  within three days of each other, and belong to accounts marked as the user's
  own. Ask the user to confirm each suggested match.
- To decide: should users select one category or compare multiple categories in
  trend views?
- To decide: what should users be able to customize, save, and share?

### Categorizing transactions

- Categorization should be a separate module in the app, not just an action
  embedded in the dashboard.
- Categories are flexible and are not restricted to income or spending
  transactions.
- Let the user import a starter set of categories.
- The app should use rules to suggest or assign categories to income and
  spending transactions.
- Rules may match transaction direction or amount (for example, positive
  amounts), source, recipient/payee, or text in the description. For example,
  the user can create a Salary category and a rule matching positive amounts
  from a particular sender.
- Transactions that no rule can categorize should be available in an
  Uncategorized review list.
- Make it easy to filter uncategorized transactions and save a useful filter as
  a reusable categorization rule. Prioritize a clear, efficient review
  experience so repeated or straightforward cases are easy to process.
- Make repeated payees or senders and frequently occurring transactions easy
  to spot, and allow sorting by transaction amount.
- Let the user select and categorize multiple matching transactions together.
- When saving a filter as a rule, ask whether to apply it to existing matching
  transactions as well as future imports.
- Provide a place to review and manage existing rules, not only create rules
  from transaction filters. The user should be able to view and search rules,
  edit them, enable or disable them, and delete them. When disabling or
  deleting a rule, ask whether to keep its existing categorizations or return
  affected transactions to Uncategorized.
- The user should remain in control: they can override the group for an
  individual transaction and create categories and rules.
- Provide category management so the user can view, create, edit, and delete
  categories. If a category being deleted is assigned to transactions, ask
  whether to move those transactions to Uncategorized or another category.
- When multiple rules conflict, or a rule change would replace a manual
  transaction override, the app should ask the user what to do rather than
  silently choosing. Possible actions include prioritizing rules, keeping a
  manual override, or updating the rules.
- When editing a rule, ask whether it should also update categories on existing
  transactions.
- The categorization module should be designed to allow future extensions;
  specific future features are not in the first-version scope yet.

## Interaction principles

- Prefer asking the user to make a decision over taking an unconfigured
  automated action. User-created and enabled rules are explicit permission to
  automate matching categorizations.
- Where appropriate, repeated decision prompts may offer a "Don't ask me
  again" option.

## Functional requirements

Write requirements so they can be verified. Use identifiers such as `FR-01`
when a requirement is agreed.

| ID | Requirement | Priority | Acceptance criteria | Status |
| --- | --- | --- | --- | --- |
| FR-01 | Import transaction data exported from ING Belgium, Triodos Belgium, and vdk bank accounts, covering up to five years where available. | First version | Supported file formats, fields, and actual history availability to verify | Draft |
| FR-02 | Preview an import and require user confirmation before saving. | First version | Preview content and confirmation behavior to define | Draft |
| FR-03 | Identify possible duplicates and ask the user how to handle them. | First version | Duplicate matching and available choices to define | Draft |
| FR-04 | Report invalid rows and allow valid rows from the same file to be imported. | First version | Required fields, row-level feedback, and partial-import behavior to define | Draft |
| FR-05 | Show each account's latest known balance and date, plus a combined balance based on those latest known balances. | First version | Show dates clearly; combined balance may combine different dates | Draft |
| FR-06 | Show monthly spending by group, total incoming money, and saving (income minus spending), excluding confirmed internal transfers; allow income such as salary to be viewed by category. | First version | Exact calculations and acceptance criteria to define | Draft |
| FR-07 | Default the dashboard to the rolling last 12 months and allow the user to select other ranges, including multi-year monthly trends where history is available. | First version | Available ranges and acceptance criteria to define | Draft |
| FR-08 | Assign income and spending transactions to categories using rules based on transaction details, while allowing per-transaction overrides and user-created categories and rules. | First version | Rule behavior and acceptance criteria to define | Draft |
| FR-09 | Ask the user to resolve conflicts between matching rules or between a rule change and a manual override; offer prioritizing rules, keeping a manual override, or updating rules. | First version | Conflict flow and effects of each choice to define | Draft |
| FR-10 | When a rule is edited, ask whether it should also update categories on existing transactions. | First version | Confirmation scope and preview behavior to define | Draft |
| FR-11 | Suggest transfers between accounts marked as the user's own when transactions have equal amounts in opposite directions within three days; require user confirmation and show confirmed transfers separately from income and spending. | First version | Account ownership setup and transfer suggestion behavior to define | Draft |
| FR-12 | Show changes in account balances over time. | First version | Trend calculation and presentation to define | Draft |
| FR-13 | Make the app accessible from devices on the user's private home network. | First version | To define in technical design | Draft |
| FR-14 | If an import has no account balance, let the user enter the current balance and its date, then calculate historical balances from that point using imported transactions. | First version | Use available history; mark balances affected by unresolved gaps as incomplete per FR-15 | Draft |
| FR-15 | If transaction history has a gap, continue with a best-effort calculation and show a non-blocking issue. Let the user remind later, upload files to close the gap, or explicitly ignore it; ignoring closes the issue and removes the incomplete marker from affected balances. | First version | Gap detection, action state, file reconciliation, and marker behavior to define | Draft |
| FR-16 | Provide a separate categorization module with an Uncategorized review list; surface repeated payees/senders and frequent transactions, allow filtering, amount sorting, and bulk categorization, and let the user save a filter as a rule. | First version | Review workflow, filter behavior, and rule creation criteria to define | Draft |
| FR-17 | When saving a filter as a rule, ask whether to apply it to existing matching transactions. | First version | Confirmation and application behavior to define | Draft |
| FR-18 | Provide a place in the categorization module to view/search, edit, enable/disable, and delete categorization rules. | First version | Rule list behavior and confirmation flow for existing categorizations to define | Draft |
| FR-19 | Let the user view, create, edit, and delete categories; when deleting a category in use, ask whether to move its transactions to Uncategorized or another category. | First version | Category management and reassignment flow to define | Draft |
| FR-20 | Allow categories to be used flexibly for income or spending transactions. | First version | Category behavior in dashboard calculations to define | Draft |
| FR-21 | Let the user import a starter set of categories. | First version | Starter set contents and import behavior to define | Draft |
| FR-22 | Prefer asking for user decisions over unconfigured automation and, where appropriate, offer a "Don't ask me again" option. | First version | Prompts and persistent preference scope to define | Draft |

## Data and user controls

- What account, transaction, and dashboard information should be stored?
- How should duplicate, pending, reversed, or uncategorized transactions appear?
- Can users edit, delete, export, or fully clear their data?
- How should users know when an import or dashboard update has completed?
- To decide

## Quality expectations

Define user-visible expectations for accessibility, usability, performance,
reliability, and supported browsers/devices.

- To decide

## Open questions

- Can each bank provide five years of transaction data in a format suitable for
  import? ING's public export formats/history and vdk's structured export format
  are unconfirmed.
- Is importing files sufficient, or is a direct bank connection eventually
  needed?
- Should household members eventually have separate access, shared access, or
  both?
- What data retention, correction, export, and deletion controls are required?
