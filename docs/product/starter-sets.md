# Starter categories, rules, and import format

This document defines the first-version built-in personal-finance starter set
and its portable import format. Starter items are optional; nothing is added
without the user's selection and confirmation.

## Built-in starter categories

Categories are flat labels, not a hierarchy, and are not restricted to income
or spending transactions. The Income and Spending headings below only group
the list for readability.

### Income

- Salary
- Benefits
- Interest income
- Refunds
- Other income

### Spending

- Housing
- Utilities
- Groceries
- Restaurants and cafes
- Transport
- Vehicle expenses
- Health
- Insurance
- Household
- Clothing
- Personal care
- Education
- Communications
- Subscriptions
- Entertainment
- Travel
- Taxes
- Bank fees
- Gifts and donations
- Other expenses

**Uncategorized** is a built-in system category. It is not imported from a
starter set. Confirmed internal transfers do not need a category.

## Conservative starter rules

Starter rules match only incoming/outgoing direction and text in the
transaction description. They do not match solely on amount, account number,
counterparty identity, or bank-specific transaction codes. Text matching is
case-insensitive. `contains_any` matches when the description contains at
least one listed phrase.

| Rule | Category | Conditions (all must match) |
| --- | --- | --- |
| Salary description terms | Salary | Direction is incoming; description contains any of `salary`, `salaris`, `loon`, `wedde`, `salaire` |
| Interest description terms | Interest income | Direction is incoming; description contains any of `interest`, `rente`, `interet`, `intérêt` |
| Refund description terms | Refunds | Direction is incoming; description contains any of `refund`, `terugbetaling`, `terugstorting`, `remboursement` |
| Bank fee description terms | Bank fees | Direction is outgoing; description contains any of `bank fee`, `bank fees`, `bankkosten`, `rekeningkosten`, `frais bancaires`, `frais de compte` |

These are deliberately limited, language-aware suggestions and may not match
the wording used by a particular bank. The user can opt out of all rules or
select rules individually. Before applying selected rules, show their existing
matches and require confirmation. Existing manual categories and conflicting
rules follow the normal categorization prompts. Only confirmed selected rules
are enabled for future transactions.

## File import format

Use UTF-8 JSON files with the `.json` extension. Version 1 has this shape:

```json
{
  "formatVersion": 1,
  "name": "Personal finance starter set",
  "categories": [
    "Salary",
    "Other income",
    "Groceries",
    "Other expenses"
  ],
  "rules": [
    {
      "name": "Salary description terms",
      "category": "Salary",
      "combine": "all",
      "conditions": [
        {
          "field": "direction",
          "operator": "equals",
          "value": "incoming"
        },
        {
          "field": "description",
          "operator": "contains_any",
          "values": ["salary", "salaris", "loon", "wedde", "salaire"]
        }
      ]
    }
  ]
}
```

### Validation and import behavior

- `formatVersion` must be the integer `1`; reject unsupported versions.
- `name`, `categories`, and `rules` are required. Categories must be nonempty
  strings and unique after trimming and case-insensitive comparison.
- For version 1, each rule must have a nonempty name, reference a category in
  the file or an existing category, and contain at least one condition.
  `combine` is `all` (AND) or `any` (OR). The supported conditions are
  `direction` with `equals` and an `incoming` or `outgoing` value, and
  `description` with `contains_any` and a nonempty array of text values.
  Description matching is case-insensitive. This is a starter-set format, not
  a full rule backup format.
- Reject malformed JSON, unknown fields, invalid values, duplicate rule names
  within the file (case-insensitive after trimming), and unsupported rule
  fields or operators before changing app data.
- Match an imported category to an existing category by trimmed,
  case-insensitive name; reuse it rather than creating a duplicate.
- Let the user select categories and rules independently, and let them select
  starter rules individually. A rule can be selected only if its category
  already exists or is also selected for import.
- Preview the categories to add/reuse and the existing transactions matched by
  each selected rule. Applying a selected rule set requires confirmation;
  selected rules are enabled after confirmation.
- Do not import transactions or bank data from a starter-set file. Do not
  change existing app data if the file is invalid or the user cancels.

