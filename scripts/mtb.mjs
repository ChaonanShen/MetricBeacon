import { createHash } from 'node:crypto';
import {
  chmodSync,
  existsSync,
  mkdirSync,
  readFileSync,
  renameSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { basename, dirname, join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const modulePath = fileURLToPath(import.meta.url);
export const repositoryRoot = resolve(dirname(modulePath), '..');

const managedKeys = [
  'MTB_WORKTREE_ID',
  'MTB_WORKTREE_SLOT',
  'MTB_BIND_HOST',
  'MTB_BROWSER_HOST',
  'GRAFANA_ADMIN_USER',
  'GRAFANA_ADMIN_PASSWORD',
];
const legacyKeys = ['MTB_AI_CORE_ENDPOINT', 'MTB_ASSISTANT_MCP_ENDPOINT', 'MTB_GRAFANA_URL'];
const managedComment = '# Mini Torchbearing worktree configuration (managed by ./scripts/mtb init)';

function unquote(value) {
  const trimmed = value.trim();
  if (trimmed.startsWith("'") && trimmed.endsWith("'")) return trimmed.slice(1, -1);
  if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return trimmed.slice(1, -1).replace(/\\([nrt"\\])/g, (_, character) => ({ n: '\n', r: '\r', t: '\t', '"': '"', '\\': '\\' })[character]);
  }
  return trimmed.replace(/\s+#.*$/, '').trim();
}

export function parseDotenv(content) {
  const result = {};
  for (const [index, rawLine] of content.split(/\r?\n/).entries()) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const candidate = line.startsWith('export ') ? line.slice(7).trim() : line;
    const separator = candidate.indexOf('=');
    if (separator <= 0) throw new Error(`invalid .env line ${index + 1}`);
    const key = candidate.slice(0, separator).trim();
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) throw new Error(`invalid .env key on line ${index + 1}`);
    result[key] = unquote(candidate.slice(separator + 1));
  }
  return result;
}

function readDotenv(path) {
  return existsSync(path) ? parseDotenv(readFileSync(path, 'utf8')) : {};
}

function isLinkedWorktree(root) {
  const gitPath = join(root, '.git');
  return existsSync(gitPath) && statSync(gitPath).isFile();
}

function parseSlot(value) {
  if (!/^(?:[0-9])$/.test(String(value ?? ''))) throw new Error('MTB_WORKTREE_SLOT must be an integer from 0 to 9');
  return Number(value);
}

function validateID(value) {
  if (!/^[a-z0-9][a-z0-9-]{0,31}$/.test(value ?? '')) {
    throw new Error('MTB_WORKTREE_ID must match [a-z0-9][a-z0-9-]{0,31}');
  }
  return value;
}

function nonEmpty(value, key, fallback) {
  const result = String(value ?? fallback ?? '').trim();
  if (!result) throw new Error(`${key} must not be empty`);
  return result;
}

export function loadConfig({ root = repositoryRoot, environment = process.env, configFile = join(root, '.env') } = {}) {
  const fromFile = readDotenv(configFile);
  const merged = { ...fromFile, ...Object.fromEntries(Object.entries(environment).filter(([, value]) => value !== undefined)) };
  const hasIdentity = merged.MTB_WORKTREE_ID !== undefined || merged.MTB_WORKTREE_SLOT !== undefined;
  if (isLinkedWorktree(root) && !hasIdentity) {
    throw new Error('linked worktree is not initialized; run ./scripts/mtb init --slot N');
  }
  if (hasIdentity && (merged.MTB_WORKTREE_ID === undefined || merged.MTB_WORKTREE_SLOT === undefined)) {
    throw new Error('MTB_WORKTREE_ID and MTB_WORKTREE_SLOT must be configured together');
  }

  const id = validateID(merged.MTB_WORKTREE_ID ?? 'main');
  const slot = parseSlot(merged.MTB_WORKTREE_SLOT ?? '0');
  const bindHost = nonEmpty(merged.MTB_BIND_HOST, 'MTB_BIND_HOST', '127.0.0.1');
  const browserHost = nonEmpty(merged.MTB_BROWSER_HOST, 'MTB_BROWSER_HOST', id === 'main' ? 'localhost' : `${id}.localhost`);
  const portOffset = slot * 100;
  return {
    root,
    configFile,
    id,
    slot,
    bindHost,
    browserHost,
    grafanaPort: 3000 + portOffset,
    aiCorePort: 8080 + portOffset,
    mcpPort: 8081 + portOffset,
    grafanaURL: `http://${browserHost}:${3000 + portOffset}`,
    grafanaAdminUser: nonEmpty(merged.GRAFANA_ADMIN_USER, 'GRAFANA_ADMIN_USER', 'admin'),
    grafanaAdminPassword: nonEmpty(merged.GRAFANA_ADMIN_PASSWORD, 'GRAFANA_ADMIN_PASSWORD', 'admin'),
    deepSeekAPIKey: String(merged.DEEPSEEK_API_KEY ?? '').trim(),
    deepSeekBaseURL: nonEmpty(merged.DEEPSEEK_BASE_URL, 'DEEPSEEK_BASE_URL', 'https://api.deepseek.com'),
    deepSeekModel: nonEmpty(merged.DEEPSEEK_MODEL, 'DEEPSEEK_MODEL', 'deepseek-v4-flash'),
    runtimeDir: join(root, '.runtime'),
    environment: merged,
  };
}

export function slugifyWorktreeName(root) {
  const directory = basename(root);
  const source = directory === 'mini-torchbearing' ? 'main' : directory.replace(/^mini-torchbearing[-_]?/i, '');
  const slug = source.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 32);
  return validateID(slug || 'worktree');
}

function listWorktreePaths(root) {
  const result = spawnSync('git', ['worktree', 'list', '--porcelain'], { cwd: root, encoding: 'utf8' });
  if (result.status !== 0) throw new Error(result.stderr.trim() || 'git worktree list failed');
  return result.stdout.split(/\r?\n/).filter((line) => line.startsWith('worktree ')).map((line) => line.slice(9));
}

function assertNoWorktreeConflict({ root, id, slot, paths = listWorktreePaths(root) }) {
  for (const path of paths) {
    if (resolve(path) === resolve(root)) continue;
    const values = readDotenv(join(path, '.env'));
    if (values.MTB_WORKTREE_ID === id) throw new Error(`worktree ID ${id} is already used by ${path}`);
    if (values.MTB_WORKTREE_SLOT !== undefined && Number(values.MTB_WORKTREE_SLOT) === slot) {
      throw new Error(`worktree slot ${slot} is already used by ${path}`);
    }
  }
}

function stripManagedConfiguration(content) {
  return content.split(/\r?\n/).filter((line) => {
    if (line.trim() === managedComment) return false;
    const match = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=/);
    return !match || (!managedKeys.includes(match[1]) && !legacyKeys.includes(match[1]));
  }).join('\n').replace(/\n+$/, '');
}

export function initializeConfig({ root = repositoryRoot, id = slugifyWorktreeName(root), slot, force = false, worktreePaths } = {}) {
  const validatedID = validateID(id);
  const validatedSlot = parseSlot(slot);
  assertNoWorktreeConflict({ root, id: validatedID, slot: validatedSlot, paths: worktreePaths });
  const path = join(root, '.env');
  const existing = existsSync(path) ? readFileSync(path, 'utf8') : '';
  const values = existing ? parseDotenv(existing) : {};
  const existingManaged = managedKeys.filter((key) => values[key] !== undefined);
  if (existingManaged.length > 0 && !force) {
    throw new Error(`worktree configuration already exists (${existingManaged.join(', ')}); use --force to replace managed keys`);
  }
  const preserved = stripManagedConfiguration(existing);
  const browserHost = validatedID === 'main' ? 'localhost' : `${validatedID}.localhost`;
  const block = [
    managedComment,
    `MTB_WORKTREE_ID=${validatedID}`,
    `MTB_WORKTREE_SLOT=${validatedSlot}`,
    'MTB_BIND_HOST=127.0.0.1',
    `MTB_BROWSER_HOST=${browserHost}`,
    'GRAFANA_ADMIN_USER=admin',
    'GRAFANA_ADMIN_PASSWORD=admin',
  ].join('\n');
  const output = `${preserved ? `${preserved}\n\n` : ''}${block}\n`;
  const temporary = `${path}.tmp-${process.pid}`;
  writeFileSync(temporary, output, { mode: 0o600 });
  chmodSync(temporary, 0o600);
  renameSync(temporary, path);
  return loadConfig({ root, environment: {}, configFile: path });
}

export function formatConfig(config) {
  return [
    `worktree=${config.id}`,
    `slot=${config.slot}`,
    `grafana=${config.grafanaURL}`,
    `aiCore=http://${config.bindHost}:${config.aiCorePort}`,
    `assistantMCP=http://${config.bindHost}:${config.mcpPort}/mcp`,
    `runtimeDir=${config.runtimeDir}`,
    `deepSeekAPIKey=${config.deepSeekAPIKey ? '<configured>' : '<absent>'}`,
  ].join('\n');
}

export function dependencyFingerprint(frontendDirectory) {
  return createHash('sha256').update(readFileSync(join(frontendDirectory, 'package-lock.json'))).digest('hex');
}

export function frontendDependenciesNeedInstall(frontendDirectory) {
  const marker = join(frontendDirectory, 'node_modules', '.mtb-package-lock.sha256');
  return !existsSync(join(frontendDirectory, 'node_modules', '.bin', 'tsc'))
    || !existsSync(marker)
    || readFileSync(marker, 'utf8').trim() !== dependencyFingerprint(frontendDirectory);
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { stdio: 'inherit', ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} exited with status ${result.status}`);
}

function capture(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: 'utf8', ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(result.stderr.trim() || `${command} exited with status ${result.status}`);
  return result.stdout.trim();
}

export function ensureFrontendDependencies(root = repositoryRoot) {
  const frontend = join(root, 'apps', 'grafana-plugin', 'frontend');
  if (!frontendDependenciesNeedInstall(frontend)) return false;
  run('npm', ['ci'], { cwd: frontend });
  const marker = join(frontend, 'node_modules', '.mtb-package-lock.sha256');
  writeFileSync(marker, `${dependencyFingerprint(frontend)}\n`);
  return true;
}

export function checkToolchain({ root = repositoryRoot, includeDocker = true } = {}) {
  const expected = { go: 'go1.26.5', node: 'v22.23.1', npm: '10.9.8' };
  const actual = {
    go: capture('go', ['env', 'GOVERSION'], { cwd: root }),
    node: capture('node', ['--version'], { cwd: root }),
    npm: capture('npm', ['--version'], { cwd: root }),
  };
  for (const key of Object.keys(expected)) {
    if (actual[key] !== expected[key]) throw new Error(`${key} version mismatch: expected ${expected[key]}, got ${actual[key]}`);
  }
  if (includeDocker) {
    capture('docker', ['info'], { cwd: root });
    capture('docker', ['compose', 'version'], { cwd: root });
    capture('docker', ['buildx', 'version'], { cwd: root });
  }
  return actual;
}

function parseInitArguments(args) {
  let id;
  let slot;
  let force = false;
  for (let index = 0; index < args.length; index += 1) {
    if (args[index] === '--name') id = args[++index];
    else if (args[index] === '--slot') slot = args[++index];
    else if (args[index] === '--force') force = true;
    else throw new Error(`unknown init argument: ${args[index]}`);
  }
  if (slot === undefined) throw new Error('init requires --slot N');
  return { id, slot, force };
}

function printUsage() {
  process.stdout.write(`usage: ./scripts/mtb <command>\n\ncommands:\n  init --slot N [--name ID] [--force]\n  config show|check\n  doctor\n  deps\n\nThe up/verify/Compose lifecycle commands are added by the next implementation gate.\n`);
}

export function main(args = process.argv.slice(2)) {
  const command = args.shift();
  if (!command || command === 'help' || command === '--help') {
    printUsage();
    return;
  }
  if (command === 'init') {
    const options = parseInitArguments(args);
    const config = initializeConfig({ root: repositoryRoot, id: options.id ?? slugifyWorktreeName(repositoryRoot), slot: options.slot, force: options.force });
    process.stdout.write(`${formatConfig(config)}\n`);
    return;
  }
  if (command === 'config') {
    const action = args.shift();
    if (args.length || !['show', 'check'].includes(action)) throw new Error('config requires show or check');
    const config = loadConfig();
    assertNoWorktreeConflict({ root: repositoryRoot, id: config.id, slot: config.slot });
    if (action === 'show') process.stdout.write(`${formatConfig(config)}\n`);
    else process.stdout.write(`worktree configuration is valid: ${config.id} (slot ${config.slot})\n`);
    return;
  }
  if (command === 'doctor') {
    const versions = checkToolchain();
    process.stdout.write(`toolchain is ready: go=${versions.go} node=${versions.node} npm=${versions.npm}\n`);
    return;
  }
  if (command === 'deps') {
    checkToolchain({ includeDocker: false });
    const installed = ensureFrontendDependencies();
    process.stdout.write(installed ? 'frontend dependencies installed\n' : 'frontend dependencies are current\n');
    return;
  }
  throw new Error(`unknown command: ${command}`);
}

if (process.argv[1] && resolve(process.argv[1]) === modulePath) {
  try {
    main();
  } catch (error) {
    process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  }
}
