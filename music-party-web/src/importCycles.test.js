import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, extname, relative, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const srcRoot = resolve(process.cwd(), 'src');
const sourceFiles = collectSourceFiles(srcRoot);

const fileSet = new Set(sourceFiles.map((file) => normalize(file)));
const importPattern = /import\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]/g;

function normalize(path) {
  return path.replaceAll('\\', '/');
}

function collectSourceFiles(dir) {
  return readdirSync(dir).flatMap((entry) => {
    const path = resolve(dir, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) return collectSourceFiles(path);
    if (!/\.(js|vue)$/.test(entry) || entry.endsWith('.test.js')) return [];
    return [path];
  });
}

function resolveImport(fromFile, specifier) {
  if (!specifier.startsWith('.')) return null;

  const base = resolve(dirname(fromFile), specifier);
  const candidates = extname(base)
    ? [base]
    : [`${base}.js`, `${base}.vue`, resolve(base, 'index.js')];

  return candidates.map(normalize).find((candidate) => fileSet.has(candidate)) || null;
}

function formatCycle(cycle) {
  return cycle.map((file) => relative(srcRoot, file)).join(' -> ');
}

function findCycles(graph) {
  const visited = new Set();
  const stack = [];
  const inStack = new Set();
  const cycles = [];

  function visit(file) {
    visited.add(file);
    stack.push(file);
    inStack.add(file);

    for (const dep of graph.get(file) || []) {
      if (!visited.has(dep)) {
        visit(dep);
      } else if (inStack.has(dep)) {
        cycles.push([...stack.slice(stack.indexOf(dep)), dep]);
      }
    }

    stack.pop();
    inStack.delete(file);
  }

  for (const file of graph.keys()) {
    if (!visited.has(file)) visit(file);
  }

  return cycles;
}

describe('frontend import graph', () => {
  it('does not contain static import cycles inside src', () => {
    const graph = new Map();

    for (const file of sourceFiles) {
      const source = readFileSync(file, 'utf8');
      const deps = [];
      for (const match of source.matchAll(importPattern)) {
        const resolved = resolveImport(file, match[1]);
        if (resolved) deps.push(resolved);
      }
      graph.set(normalize(file), deps);
    }

    const cycles = findCycles(graph);
    expect(cycles.map(formatCycle)).toEqual([]);
  });
});
