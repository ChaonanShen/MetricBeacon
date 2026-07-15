import assert from 'node:assert/strict';
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';

import {
  composeFiles,
  composeProjectName,
  dependencyFingerprint,
  formatConfig,
  frontendDependenciesNeedInstall,
  initializeConfig,
  loadConfig,
  parseDotenv,
  parsePublishedPort,
  slugifyWorktreeName,
} from '../../scripts/mtb.mjs';

function temporaryRoot(t, name = 'mini-torchbearing') {
  const parent = join(process.env.TMPDIR ?? '/tmp', `mtb-config-${process.pid}-${Math.random().toString(16).slice(2)}`);
  const root = join(parent, name);
  mkdirSync(join(root, '.git'), { recursive: true });
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  return root;
}

test('parseDotenv accepts the repository dotenv subset without evaluating values', () => {
  assert.deepEqual(parseDotenv('A=plain\nB="two words"\nC=shown # comment\nexport D=ok\n'), {
    A: 'plain', B: 'two words', C: 'shown', D: 'ok',
  });
});

test('primary worktree defaults to main slot zero', (t) => {
  const root = temporaryRoot(t);
  const config = loadConfig({ root, environment: {} });
  assert.equal(config.id, 'main');
  assert.equal(config.grafanaURL, 'http://localhost:3000');
  assert.equal(config.aiCorePort, 8080);
  assert.equal(config.mcpPort, 8081);
});

test('linked worktree requires explicit initialization', (t) => {
  const root = temporaryRoot(t, 'mini-torchbearing-feature-a');
  rmSync(join(root, '.git'), { recursive: true });
  writeFileSync(join(root, '.git'), 'gitdir: /tmp/example\n');
  assert.throws(() => loadConfig({ root, environment: {} }), /not initialized/);
});

test('slot derives stable ports and a unique browser hostname', (t) => {
  const root = temporaryRoot(t, 'mini-torchbearing-feature-a');
  writeFileSync(join(root, '.env'), 'MTB_WORKTREE_ID=feature-a\nMTB_WORKTREE_SLOT=2\n');
  const config = loadConfig({ root, environment: {} });
  assert.equal(config.grafanaURL, 'http://feature-a.localhost:3200');
  assert.equal(config.aiCorePort, 8280);
  assert.equal(config.mcpPort, 8281);
});

test('init migrates legacy keys while preserving model credentials', (t) => {
  const root = temporaryRoot(t, 'mini-torchbearing-feature-a');
  writeFileSync(join(root, '.env'), 'DEEPSEEK_API_KEY=secret-value\nMTB_GRAFANA_URL=http://localhost:3000\n');
  const config = initializeConfig({ root, id: 'feature-a', slot: 1, worktreePaths: [root] });
  const content = readFileSync(join(root, '.env'), 'utf8');
  assert.match(content, /DEEPSEEK_API_KEY=secret-value/);
  assert.doesNotMatch(content, /MTB_GRAFANA_URL/);
  assert.equal(config.grafanaURL, 'http://feature-a.localhost:3100');
  assert.doesNotMatch(formatConfig(config), /secret-value/);
});

test('dependency fingerprint detects absent and current installs', (t) => {
  const root = temporaryRoot(t, 'frontend');
  writeFileSync(join(root, 'package-lock.json'), '{"lockfileVersion":3}\n');
  assert.equal(frontendDependenciesNeedInstall(root), true);
  mkdirSync(join(root, 'node_modules', '.bin'), { recursive: true });
  writeFileSync(join(root, 'node_modules', '.bin', 'tsc'), '');
  writeFileSync(join(root, 'node_modules', '.mtb-package-lock.sha256'), `${dependencyFingerprint(root)}\n`);
  assert.equal(frontendDependenciesNeedInstall(root), false);
  writeFileSync(join(root, 'package-lock.json'), '{"lockfileVersion":4}\n');
  assert.equal(frontendDependenciesNeedInstall(root), true);
});

test('worktree names are normalized into Compose-safe identifiers', () => {
  assert.equal(slugifyWorktreeName('/tmp/mini-torchbearing-Feature_A'), 'feature-a');
});

test('Compose resources are named by worktree, purpose, mode, and run', (t) => {
  const root = temporaryRoot(t, 'mini-torchbearing-feature-a');
  writeFileSync(join(root, '.env'), 'MTB_WORKTREE_ID=feature-a\nMTB_WORKTREE_SLOT=1\n');
  const config = loadConfig({ root, environment: {} });
  assert.equal(composeProjectName(config, 'dev', 'mock'), 'mini-torchbearing-feature-a-dev-mock');
  assert.equal(composeProjectName(config, 'e2e', 'real-metrics', 'run-1'), 'mini-torchbearing-feature-a-e2e-real-metrics-run-1');
  assert.deepEqual(composeFiles('/repo', 'real-agent'), [
    '/repo/compose.mock-e2e.yaml', '/repo/compose.real-metrics-e2e.yaml', '/repo/compose.real-agent-e2e.yaml',
  ]);
  assert.deepEqual(composeFiles('/repo', 'incident'), [
    '/repo/compose.mock-e2e.yaml', '/repo/compose.real-metrics-e2e.yaml', '/repo/compose.incident-e2e.yaml',
  ]);
});

test('published Docker port parsing rejects missing and zero ports', () => {
  assert.equal(parsePublishedPort('127.0.0.1:49152\n'), 49152);
  assert.throws(() => parsePublishedPort('127.0.0.1:0'), /could not parse/);
  assert.throws(() => parsePublishedPort(''), /could not parse/);
});
