#!/usr/bin/env node
// Usage: node scripts/bump.js [major|minor|patch]
const fs = require('fs');
const path = require('path');

const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
const [major, minor, patch] = pkg.version.split('.').map(Number);
const type = process.argv[2] || 'patch';

const next = type === 'major' ? `${major+1}.0.0`
           : type === 'minor' ? `${major}.${minor+1}.0`
           : `${major}.${minor}.${patch+1}`;

pkg.version = next;
for (const k of Object.keys(pkg.optionalDependencies || {})) {
  pkg.optionalDependencies[k] = next;
}
fs.writeFileSync(path.join(__dirname, '..', 'package.json'), JSON.stringify(pkg, null, 2) + '\n');
console.log(next);
