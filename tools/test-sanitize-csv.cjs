const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const page = fs.readFileSync(path.join(__dirname, "sanitize-csv.html"), "utf8");
const script = page.match(/<script>([\s\S]*?)<\/script>/);
assert.ok(script, "sanitizer page should contain its local script");
const context = { window: {} };
vm.runInNewContext(script[1], context);
const { parseCsv, prepareRows, sanitizeRows, serializeCsv } = context.window.CsvSanitizer;

test("parses semicolon CSV with quoted delimiters and multiline values", () => {
  const rows = parseCsv('A;B\r\n"one;two";"line one\nline two"\r\n', ";");
  assert.equal(
    JSON.stringify(rows),
    JSON.stringify([["A", "B"], ["one;two", "line one\nline two"]])
  );
});

test("redacts values by default and preserves only explicitly kept columns", () => {
  const rows = [
    ["Rekeningnummer", "Bedrag", "Omschrijving"],
    ["123456", "-10,00", "Persoonlijke betaling"],
    ["123456", "+10,00", "Persoonlijke betaling"],
  ];
  const result = sanitizeRows(rows, new Set([1]));
  assert.equal(
    JSON.stringify(result.rows),
    JSON.stringify([
      rows[0],
      ["REDACTED_0001", "-10,00", "REDACTED_0001"],
      ["REDACTED_0001", "+10,00", "REDACTED_0001"],
    ])
  );
  assert.equal(result.dataRowCount, 2);
});

test("headerless files treat the first record as data and generate safe column names", () => {
  const rows = [
    ["date-value", "account-value", "amount-value"],
    ["second-date", "second-account", "second-amount"],
  ];
  const prepared = prepareRows(rows, false, 1);
  assert.equal(
    JSON.stringify(prepared),
    JSON.stringify([
      ["Column 1", "Column 2", "Column 3"],
      ...rows,
    ])
  );
  const sanitized = sanitizeRows(prepared, new Set());
  assert.equal(sanitized.rows[0][0], "Column 1");
  assert.match(sanitized.rows[1][0], /^REDACTED_/);
  assert.notEqual(sanitized.rows[1][0], "date-value");
});

test("header mode skips any records before the selected header", () => {
  const rows = [
    ["metadata"],
    ["Column A", "Column B"],
    ["value-a", "value-b"],
  ];
  assert.equal(
    JSON.stringify(prepareRows(rows, true, 2)),
    JSON.stringify([rows[1], rows[2]])
  );
});

test("serialized sanitized output can be parsed back without data loss", () => {
  const rows = [["A", "B"], ["value;with delimiter", 'value "quoted"']];
  const serialized = serializeCsv(rows, ";");
  assert.equal(JSON.stringify(parseCsv(serialized, ";")), JSON.stringify(rows));
});

test("rejects malformed quoted input", () => {
  assert.throws(() => parseCsv('A;B\n"unclosed;value\n', ";"), /unclosed quoted field/);
});
