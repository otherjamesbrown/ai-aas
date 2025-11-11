#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

const THRESHOLDS = {
  lines: 80,
  statements: 80,
  functions: 80,
  branches: 70,
};

const summaryPath = path.resolve(__dirname, '../coverage/coverage-final.json');

if (!fs.existsSync(summaryPath)) {
  console.error(`Coverage summary not found at ${summaryPath}`);
  process.exit(1);
}

const raw = JSON.parse(fs.readFileSync(summaryPath, 'utf-8'));

const totals = {
  lines: { total: 0, covered: 0 },
  statements: { total: 0, covered: 0 },
  functions: { total: 0, covered: 0 },
  branches: { total: 0, covered: 0 },
};

for (const [filePath, fileStats] of Object.entries(raw)) {
  if (!filePath.includes(`${path.sep}src${path.sep}`)) {
    continue;
  }

  const statementIds = Object.keys(fileStats.statementMap || {});
  totals.statements.total += statementIds.length;
  totals.statements.covered += statementIds.filter((id) => (fileStats.s?.[id] ?? 0) > 0).length;

  const functionIds = Object.keys(fileStats.fnMap || {});
  totals.functions.total += functionIds.length;
  totals.functions.covered += functionIds.filter((id) => (fileStats.f?.[id] ?? 0) > 0).length;

  const branchEntries = Object.values(fileStats.b || {});
  for (const counts of branchEntries) {
    const branches = Array.isArray(counts) ? counts : [];
    totals.branches.total += branches.length;
    totals.branches.covered += branches.filter((count) => count > 0).length;
  }

  const lineNumbers = new Set();
  for (const meta of Object.values(fileStats.statementMap || {})) {
    lineNumbers.add(meta.start.line);
  }
  const coveredLines = new Set();
  for (const [id, count] of Object.entries(fileStats.s || {})) {
    if ((count || 0) > 0) {
      const meta = fileStats.statementMap?.[id];
      if (meta) {
        coveredLines.add(meta.start.line);
      }
    }
  }
  totals.lines.total += lineNumbers.size;
  totals.lines.covered += coveredLines.size;
}

const percentages = Object.fromEntries(
  Object.entries(totals).map(([key, value]) => {
    const pct = value.total === 0 ? 100 : (value.covered / value.total) * 100;
    return [key, pct];
  }),
);

let failed = false;
for (const [metric, target] of Object.entries(THRESHOLDS)) {
  const value = percentages[metric];
  if (typeof value !== 'number') {
    console.error(`Missing coverage metric "${metric}" in summary`);
    failed = true;
    continue;
  }
  if (value < target) {
    console.error(`Coverage for ${metric} is ${value}% (target ${target}%)`);
    failed = true;
  }
}

if (failed) {
  process.exit(1);
}

console.log('Coverage thresholds met:', Object.entries(THRESHOLDS).map(([k, v]) => `${k} >= ${v}%`).join(', '));

