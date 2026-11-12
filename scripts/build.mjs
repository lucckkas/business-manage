#!/usr/bin/env node
// Reproducible, runtime-free release packages. Install web dependencies first.
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
process.chdir(root);
const version = process.env.ERP_VERSION ?? '0.4.0';
if (!/^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$/.test(version))
  throw new Error('ERP_VERSION debe ser una versión semántica.');
const targets = process.argv.includes('--current')
  ? [process.platform === 'win32' ? 'windows' : 'linux']
  : ['linux', 'windows'];
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
function run(command, args, env = {}) {
  execFileSync(command, args, {
    stdio: 'inherit',
    env: { ...process.env, ...env },
    ...(process.platform === 'win32' && command === npm ? { shell: true } : {}),
  });
}
run(process.execPath, ['api/openapi.mjs']);
run(npm, ['--prefix', 'web', 'run', 'generate']);
run(npm, ['--prefix', 'web', 'run', 'build']);
run('go', ['mod', 'verify']);
mkdirSync('artifacts', { recursive: true });
for (const target of targets) {
  const executable = `artifacts/erp-${target}-amd64${target === 'windows' ? '.exe' : ''}`;
  run(
    'go',
    [
      'build',
      '-trimpath',
      '-buildvcs=false',
      '-ldflags',
      `-s -w -X main.version=${version}`,
      '-o',
      executable,
      './cmd/erp',
    ],
    { CGO_ENABLED: '0', GOOS: target, GOARCH: 'amd64' }
  );
  run('go', ['run', './tools/package', '-version', version, '-os', target, '-binary', executable]);
}
