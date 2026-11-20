import { expect, test } from "@playwright/test";

const salaryCSV = `"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","salary interest payment","112,50"`;
const bankFeeCSV = [
  "Rekeningnummer;Naam van de rekening;Rekening tegenpartij;Omzetnummer;Boekingsdatum;Valutadatum;Bedrag;Munteenheid;Omschrijving;Detail van de omzet;Bericht",
  "BE100;Synthetic Account;BE200;TX-1;13/03/2025;14/03/2025;-10,25;EUR;Card;bank fee monthly;Synthetic detail",
].join("\n");

test("import, starter categorization, duplicate handling, and dashboard filters", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("No transactions in this period.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Import data" })).toBeVisible();

  await page.goto("/imports");
  await page.locator("#files").setInputFiles([
    { name: "triodos.csv", mimeType: "text/csv", buffer: Buffer.from(salaryCSV) },
    {
      name: "invalid.csv",
      mimeType: "text/csv",
      buffer: Buffer.from(`"not-a-date","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","invalid row","112,50"`),
    },
  ]);
  await page.getByRole("button", { name: "Review import" }).click();
  await expect(page.getByRole("heading", { name: "Files need attention" })).toBeVisible();
  await expect(page.locator('input[name="token"]')).toHaveCount(0);
  await page.goto("/transactions");
  await expect(page.getByRole("heading", { name: "No transactions yet" })).toBeVisible();

  await page.goto("/imports");
  await page.locator("#files").setInputFiles([
    { name: "triodos.csv", mimeType: "text/csv", buffer: Buffer.from(salaryCSV) },
    { name: "ing.csv", mimeType: "text/csv", buffer: Buffer.from(bankFeeCSV) },
  ]);
  await page.getByRole("button", { name: "Review import" }).click();
  await expect(page.getByRole("heading", { name: "Review import" })).toBeVisible();
  const accountConfirmations = page.locator('input[name^="account_confirm["]');
  await expect(accountConfirmations).toHaveCount(2);
  for (const confirmation of await accountConfirmations.all()) {
    await confirmation.check();
  }
  await page.getByRole("button", { name: "Confirm and import" }).click();
  await expect(page.getByRole("heading", { name: "Import complete" })).toBeVisible();

  await page.goto("/imports");
  await page.locator("#files").setInputFiles([
    { name: "triodos-again.csv", mimeType: "text/csv", buffer: Buffer.from(salaryCSV) },
  ]);
  await page.getByRole("button", { name: "Review import" }).click();
  await expect(page.getByText("1 possible duplicates require a choice for each match.")).toBeVisible();
  await page.locator('select[name^="action["]').selectOption("skip");
  await page.locator('input[name^="account_confirm["]').check();
  await page.getByRole("button", { name: "Confirm and import" }).click();
  await expect(page.getByRole("heading", { name: "Import complete" })).toBeVisible();

  await page.goto("/settings/starter-set");
  await page.locator('input[name="category"][value="Salary"]').check();
  await page.locator('input[name="category"][value="Interest income"]').check();
  await page.locator('input[name="category"][value="Bank fees"]').check();
  await page.locator('input[name="rule"][value="Salary description terms"]').check();
  await page.locator('input[name="rule"][value="Interest description terms"]').check();
  await page.locator('input[name="rule"][value="Bank fee description terms"]').check();
  await page.getByRole("button", { name: "Review selection" }).click();
  await expect(page.getByRole("heading", { name: "Review starter selection" })).toBeVisible();
  await expect(page.getByText("salary interest payment")).toBeVisible();
  await expect(page.getByText("bank fee monthly")).toBeVisible();
  await page.getByRole("link", { name: "Change selection" }).click();
  await expect(page.locator('input[name="category"][value="Salary"]')).not.toBeChecked();
  await expect(page.locator('input[name="rule"][value="Salary description terms"]')).not.toBeChecked();

  await page.locator('input[name="category"][value="Salary"]').check();
  await page.locator('input[name="category"][value="Interest income"]').check();
  await page.locator('input[name="category"][value="Bank fees"]').check();
  await page.locator('input[name="rule"][value="Salary description terms"]').check();
  await page.locator('input[name="rule"][value="Interest description terms"]').check();
  await page.locator('input[name="rule"][value="Bank fee description terms"]').check();
  await page.getByRole("button", { name: "Review selection" }).click();
  await page.locator('select[name^="transaction_category["]').selectOption({ label: "Salary" });
  await page.getByRole("button", { name: "Confirm and apply" }).click();
  await expect(page.getByRole("status")).toContainText("Your selected categories and rules are saved.");

  await page.goto("/transactions");
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await expect(page.locator("tbody tr").filter({ hasText: "salary interest payment" })).toContainText("Salary");
  await expect(page.locator("tbody tr").filter({ hasText: "bank fee monthly" })).toContainText("Bank fees");

  await page.goto("/?range=all");
  await expect(page.locator("#monthly-chart")).toBeVisible();
  await expect(page.locator("#spending-chart")).toBeVisible();
  await expect(page.locator("#income-chart")).toBeVisible();
  const actualBalances = await page.locator("#dashboard-data").textContent()
    .then((data) => JSON.parse(data).balances);
  const salaryOption = page.locator("#category option").filter({ hasText: "Salary" });
  await page.locator("#category").selectOption(await salaryOption.getAttribute("value"));
  await page.getByRole("button", { name: "Apply filters" }).click();
  await expect(page.locator(".dashboard-summary .metric").nth(1)).toHaveText("€12.5");
  await expect(page.locator(".dashboard-summary .metric").nth(2)).toHaveText("€0");
  await expect(page.getByText("No spending in this selection.")).toBeVisible();
  await expect(page.getByText("actual account-balance history remains unchanged.")).toBeVisible();
  const filteredBalances = await page.locator("#dashboard-data").textContent()
    .then((data) => JSON.parse(data).balances);
  expect(filteredBalances).toEqual(actualBalances);

  await page.goto("/?range=all");
  await page.locator("#range").selectOption("custom");
  await expect(page.locator("#custom-range-fields")).toBeVisible();
  await page.locator("#start").fill("2025-03-12");
  await page.locator("#end").fill("2025-03-12");
  await page.getByRole("button", { name: "Apply filters" }).click();
  await expect(page.locator(".dashboard-summary .metric").nth(1)).toHaveText("€12.5");
  await expect(page.locator(".dashboard-summary .metric").nth(2)).toHaveText("€0");
});
