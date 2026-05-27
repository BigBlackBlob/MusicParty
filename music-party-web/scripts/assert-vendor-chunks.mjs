import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const distAssets = resolve(process.cwd(), 'dist/assets');
const vendorFiles = readdirSync(distAssets).filter((file) => /^vendor-.*\.js$/.test(file));
const vendorSet = new Set(vendorFiles);
const importPattern = /from\s+["']\.\/([^"']+)["']/g;
const cycles = [];

for (const file of vendorFiles) {
  const source = readFileSync(resolve(distAssets, file), 'utf8');
  const imports = [...source.matchAll(importPattern)]
    .map((match) => match[1])
    .filter((imported) => vendorSet.has(imported));

  for (const imported of imports) {
    const importedSource = readFileSync(resolve(distAssets, imported), 'utf8');
    const importsBack = [...importedSource.matchAll(importPattern)]
      .some((match) => match[1] === file);
    if (importsBack) cycles.push(`${file} <-> ${imported}`);
  }
}

if (cycles.length > 0) {
  console.error(`Vendor chunk import cycle detected:\n${[...new Set(cycles)].join('\n')}`);
  process.exit(1);
}

console.log(`Checked ${vendorFiles.length} vendor chunks; no direct vendor import cycles found.`);
