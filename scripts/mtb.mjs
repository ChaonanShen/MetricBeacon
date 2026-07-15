import { createHash } from 'node:crypto';
import {
  chmodSync,
  existsSync,
  mkdtempSync,
  mkdirSync,
  readFileSync,
  renameSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { basename, dirname, join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
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

export function composeFiles(root, mode) {
  const base = join(root, 'compose.mock-e2e.yaml');
  if (mode === 'mock') return [base];
  const metrics = join(root, 'compose.real-metrics-e2e.yaml');
  if (mode === 'real-metrics') return [base, metrics];
  if (mode === 'real-agent') return [base, metrics, join(root, 'compose.real-agent-e2e.yaml')];
  throw new Error(`unsupported mode: ${mode}`);
}

export function composeProjectName(config, purpose, mode, runID) {
  if (!['dev', 'e2e', 'diag'].includes(purpose)) throw new Error(`unsupported Compose purpose: ${purpose}`);
  const suffix = purpose === 'dev' ? '' : `-${nonEmpty(runID, 'runID')}`;
  return `mini-torchbearing-${config.id}-${purpose}-${mode}${suffix}`;
}

export function parsePublishedPort(value) {
  const match = String(value).trim().match(/:(\d+)$/);
  if (!match || Number(match[1]) <= 0) throw new Error(`could not parse published port from ${JSON.stringify(value)}`);
  return Number(match[1]);
}

function runID() {
  return `${Date.now()}-${process.pid}`;
}

function composeContext(config, { purpose, mode, identifier = runID() }) {
  const files = composeFiles(config.root, mode);
  const transient = purpose !== 'dev';
  const project = composeProjectName(config, purpose, mode, transient ? identifier : undefined);
  const args = ['compose'];
  if (existsSync(config.configFile)) args.push('--env-file', config.configFile);
  args.push('--project-directory', config.root, '-p', project);
  for (const file of files) args.push('-f', file);
  const environment = {
    ...config.environment,
    MTB_BIND_HOST: config.bindHost,
    GRAFANA_HOST_PORT: String(transient ? 0 : config.grafanaPort),
    AI_CORE_HOST_PORT: String(transient ? 0 : config.aiCorePort),
    ASSISTANT_MCP_HOST_PORT: String(transient ? 0 : config.mcpPort),
    GRAFANA_ADMIN_USER: config.grafanaAdminUser,
    GRAFANA_ADMIN_PASSWORD: config.grafanaAdminPassword,
    DEEPSEEK_API_KEY: config.deepSeekAPIKey,
    DEEPSEEK_BASE_URL: config.deepSeekBaseURL,
    DEEPSEEK_MODEL: config.deepSeekModel,
  };
  return { config, purpose, mode, project, files, args, environment };
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
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if (result.status !== 0) throw new Error(`${command} exited with status ${result.status}`);
}

function capture(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: 'utf8', maxBuffer: 32 << 20, ...options });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if (result.status !== 0) throw new Error(result.stderr.trim() || `${command} exited with status ${result.status}`);
  return result.stdout.trim();
}

function composeRun(context, args, options = {}) {
  return run('docker', [...context.args, ...args], { cwd: context.config.root, env: context.environment, ...options });
}

function composeCapture(context, args) {
  return capture('docker', [...context.args, ...args], { cwd: context.config.root, env: context.environment });
}

function composeCleanup(context, removeVolumes) {
  const args = [...context.args, 'down'];
  if (removeVolumes) args.push('-v');
  if (context.purpose !== 'dev') args.push('--rmi', 'local');
  args.push('--remove-orphans');
  const result = spawnSync('docker', args, { cwd: context.config.root, env: context.environment, stdio: 'inherit' });
  if (result.error) process.stderr.write(`cleanup failed: ${result.error.message}\n`);
  else if (result.status !== 0) process.stderr.write(`cleanup failed for Compose project ${context.project}\n`);
}

function publishedURL(context, service, containerPort, path = '') {
  const binding = composeCapture(context, ['port', service, String(containerPort)]);
  return `http://${context.config.bindHost}:${parsePublishedPort(binding)}${path}`;
}

function buildFrontend(root) {
  run('npm', ['run', 'build'], { cwd: join(root, 'apps', 'grafana-plugin', 'frontend') });
}

function ensurePlaywright(root) {
  run('npm', ['exec', '--', 'playwright', 'install', 'chromium'], { cwd: join(root, 'apps', 'grafana-plugin', 'frontend') });
}

function waitForRealMetrics(context) {
  run(join(context.config.root, 'scripts', 'wait-for-real-metrics.sh'), [context.project, context.files[0], context.files[1]], {
    cwd: context.config.root,
    env: context.environment,
  });
}

function runGrafanaE2E(context, realMetrics) {
  const grafanaURL = publishedURL(context, 'grafana', 3000);
  const environment = {
    ...context.environment,
    GRAFANA_URL: grafanaURL,
    REAL_METRICS: realMetrics ? '1' : '0',
  };
  run(join(context.config.root, 'tests', 'e2e', 'mock', 'api-e2e.sh'), [], { cwd: context.config.root, env: environment });
  run('npm', ['run', 'test:e2e'], { cwd: join(context.config.root, 'apps', 'grafana-plugin', 'frontend'), env: environment });
}

function assertRealAgentOutputSafe(context) {
  const logs = composeCapture(context, ['logs', '--no-color']);
  const temporaryDirectory = mkdtempSync(join(tmpdir(), 'mini-torchbearing-real-agent-'));
  const databasePath = join(temporaryDirectory, 'ai-core.sqlite');
  try {
    composeRun(context, ['cp', 'ai-core:/var/lib/ai-core/ai-core.sqlite', databasePath]);
    const database = readFileSync(databasePath);
    for (const marker of ['http://prometheus:9090', 'reasoning-marker', 'raw-series-marker']) {
      if (logs.includes(marker) || database.includes(Buffer.from(marker))) throw new Error('real-agent output included a prohibited marker');
    }
    if (context.config.deepSeekAPIKey) {
      const key = context.config.deepSeekAPIKey;
      if (logs.includes(key) || database.includes(Buffer.from(key))) throw new Error('real-agent output persisted the DeepSeek API key');
    }
  } finally {
    rmSync(temporaryDirectory, { recursive: true, force: true });
  }
}

function withTransientCompose(context, operation) {
  let interrupted;
  const handlers = Object.fromEntries(['SIGINT', 'SIGTERM', 'SIGHUP'].map((signal) => [signal, () => { interrupted = signal; }]));
  for (const [signal, handler] of Object.entries(handlers)) process.once(signal, handler);
  try {
    operation();
    if (interrupted) throw new Error(`interrupted by ${interrupted}`);
  } finally {
    for (const [signal, handler] of Object.entries(handlers)) process.removeListener(signal, handler);
    composeCleanup(context, true);
  }
}

function runE2E(config, mode, { frontendBuilt = false } = {}) {
  if (mode === 'real-agent' && !config.deepSeekAPIKey) throw new Error('DEEPSEEK_API_KEY is required for real-agent E2E');
  if (mode !== 'real-agent') ensurePlaywright(config.root);
  if (!frontendBuilt) buildFrontend(config.root);
  const context = composeContext(config, { purpose: 'e2e', mode });
  withTransientCompose(context, () => {
    composeRun(context, ['up', '--build', '--wait']);
    if (mode !== 'mock') waitForRealMetrics(context);
    if (mode === 'real-agent') {
      const environment = { ...context.environment, GRAFANA_URL: publishedURL(context, 'grafana', 3000) };
      run('node', [join(config.root, 'tests', 'e2e', 'real-agent', 'api-smoke.mjs')], { cwd: config.root, env: environment });
      assertRealAgentOutputSafe(context);
    } else {
      runGrafanaE2E(context, mode === 'real-metrics');
    }
  });
}

function runRealMetricsDiagnostic(config) {
  const context = composeContext(config, { purpose: 'diag', mode: 'real-metrics' });
  withTransientCompose(context, () => {
    composeRun(context, ['up', '--build', '--wait', 'prometheus', 'node-exporter', 'assistant-mcp']);
    waitForRealMetrics(context);
    run(join(config.root, 'scripts', 'probe-real-prometheus.sh'), [context.project, context.files[0], context.files[1]], {
      cwd: config.root,
      env: context.environment,
    });
    const endpoint = publishedURL(context, 'assistant-mcp', 8081, '/mcp');
    run('go', ['test', './internal/bootstrap', '-run', '^TestLivePrometheusMCPDiagnostic$', '-count=1', '-v'], {
      cwd: join(config.root, 'services', 'assistant-mcp'),
      env: { ...context.environment, MTB_RUN_LIVE_MCP_DIAGNOSTIC: '1', MTB_LIVE_MCP_ENDPOINT: endpoint },
    });
  });
}

function prepareHost(config) {
  checkToolchain();
  ensureFrontendDependencies(config.root);
}

function canBindTCP(host, port) {
  const program = [
    "const net = require('node:net');",
    'const server = net.createServer();',
    "server.once('error', () => process.exit(1));",
    'server.listen(Number(process.argv[2]), process.argv[1], () => server.close(() => process.exit(0)));',
  ].join('');
  const result = spawnSync(process.execPath, ['-e', program, host, String(port)], { stdio: 'ignore' });
  return result.status === 0;
}

function projectPublishedPort(context, service, containerPort) {
  const result = spawnSync('docker', [...context.args, 'port', service, String(containerPort)], {
    cwd: context.config.root,
    env: context.environment,
    encoding: 'utf8',
  });
  if (result.status !== 0 || !result.stdout.trim()) return undefined;
  try {
    return parsePublishedPort(result.stdout);
  } catch {
    return undefined;
  }
}

function assertDevelopmentPortsAvailable(context) {
  const bindings = [
    ['grafana', 3000, context.config.grafanaPort],
    ['ai-core', 8080, context.config.aiCorePort],
    ['assistant-mcp', 8081, context.config.mcpPort],
  ];
  for (const [service, containerPort, hostPort] of bindings) {
    if (projectPublishedPort(context, service, containerPort) === hostPort) continue;
    if (!canBindTCP(context.config.bindHost, hostPort)) {
      throw new Error(`host port ${hostPort} for ${service} is unavailable; choose another worktree slot with ./scripts/mtb init --slot N`);
    }
  }
}

function assertModeReady(config, mode) {
  composeFiles(config.root, mode);
  if (mode === 'real-agent' && !config.deepSeekAPIKey) throw new Error('DEEPSEEK_API_KEY is required when mode=real-agent');
}

function runDevUp(config, mode) {
  assertModeReady(config, mode);
  assertNoWorktreeConflict({ root: config.root, id: config.id, slot: config.slot });
  checkToolchain();
  const context = composeContext(config, { purpose: 'dev', mode });
  assertDevelopmentPortsAvailable(context);
  ensureFrontendDependencies(config.root);
  buildFrontend(config.root);
  const existed = composeCapture(context, ['ps', '-q']) !== '';
  try {
    composeRun(context, ['up', '--build', '--wait']);
  } catch (error) {
    if (!existed) composeCleanup(context, false);
    throw error;
  }
  process.stdout.write([
    '',
    `Mini Torchbearing is ready: ${config.grafanaURL}/a/mini-torchbearing-app/workbench`,
    `Grafana user: ${config.grafanaAdminUser}`,
    `Compose project: ${context.project}`,
    `Logs: ./scripts/mtb logs --mode ${mode}`,
    `Stop: ./scripts/mtb down --mode ${mode}`,
    '',
  ].join('\n'));
}

function runDevLifecycle(config, command, mode, confirmed) {
  assertModeReady(config, mode);
  const context = composeContext(config, { purpose: 'dev', mode });
  if (command === 'ps') composeRun(context, ['ps']);
  else if (command === 'logs') composeRun(context, ['logs', '-f']);
  else if (command === 'down') composeRun(context, ['down', '--remove-orphans']);
  else if (command === 'reset') {
    if (!confirmed) throw new Error(`reset deletes volume data for ${context.project}; repeat with --yes`);
    composeRun(context, ['down', '-v', '--remove-orphans']);
  } else throw new Error(`unsupported lifecycle command: ${command}`);
}

function runVerification(config, mode, full) {
  assertModeReady(config, mode);
  prepareHost(config);
  buildFrontend(config.root);
  if (full) {
    run('make', ['check'], { cwd: config.root, env: config.environment });
  } else {
    run('make', ['validate-contracts', 'boundary-check', 'test-ai-mcp', 'test-plugin-backend', 'test-frontend-unit', 'test-diagnostics'], {
      cwd: config.root,
      env: config.environment,
    });
  }
  runE2E(config, mode, { frontendBuilt: true });
}

function runE2ECommand(config, mode) {
  assertModeReady(config, mode);
  prepareHost(config);
  runE2E(config, mode);
}

function runDiagnostic(config, target) {
  if (target === 'real-metrics') {
    checkToolchain();
    runRealMetricsDiagnostic(config);
    return;
  }
  if (target === 'deepseek') {
    if (!config.deepSeekAPIKey) throw new Error('DEEPSEEK_API_KEY is required for the DeepSeek diagnostic');
    checkToolchain({ includeDocker: false });
    run('node', [join(config.root, 'tests', 'diagnostics', 'deepseek-smoke.mjs')], {
      cwd: config.root,
      env: {
        ...config.environment,
        DEEPSEEK_API_KEY: config.deepSeekAPIKey,
        DEEPSEEK_BASE_URL: config.deepSeekBaseURL,
        DEEPSEEK_MODEL: config.deepSeekModel,
      },
    });
    return;
  }
  throw new Error('diagnose requires real-metrics or deepseek');
}

function runLocalService(config, service) {
  checkToolchain({ includeDocker: false });
  mkdirSync(config.runtimeDir, { recursive: true });
  if (service === 'assistant-mcp') {
    run('go', ['run', './cmd/server'], {
      cwd: join(config.root, 'services', 'assistant-mcp'),
      env: { ...config.environment, ASSISTANT_MCP_LISTEN_ADDRESS: `${config.bindHost}:${config.mcpPort}` },
    });
    return;
  }
  if (service === 'ai-core') {
    const configuredProfile = config.environment.AI_CORE_AGENT_PROFILE_PATH;
    const profile = configuredProfile
      ? (configuredProfile.startsWith('/') ? configuredProfile : resolve(config.root, configuredProfile))
      : join(config.root, 'data', 'agent-knowledge', 'node_exporter.md');
    run('go', ['run', './cmd/server'], {
      cwd: join(config.root, 'services', 'ai-core'),
      env: {
        ...config.environment,
        AI_CORE_LISTEN_ADDRESS: `${config.bindHost}:${config.aiCorePort}`,
        AI_CORE_SQLITE_PATH: join(config.runtimeDir, 'ai-core.sqlite'),
        AI_CORE_AGENT_PROFILE_PATH: profile,
        ASSISTANT_MCP_ENDPOINT: `http://${config.bindHost}:${config.mcpPort}/mcp`,
        DEEPSEEK_API_KEY: config.deepSeekAPIKey,
        DEEPSEEK_BASE_URL: config.deepSeekBaseURL,
        DEEPSEEK_MODEL: config.deepSeekModel,
      },
    });
    return;
  }
  throw new Error('run requires assistant-mcp or ai-core');
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

function parseCommandOptions(args, { allowFull = false, allowYes = false } = {}) {
  let mode = 'mock';
  let full = false;
  let confirmed = false;
  for (let index = 0; index < args.length; index += 1) {
    if (args[index] === '--mode') {
      if (index + 1 >= args.length) throw new Error('--mode requires a value');
      mode = args[++index];
    } else if (allowFull && args[index] === '--full') full = true;
    else if (allowYes && args[index] === '--yes') confirmed = true;
    else throw new Error(`unknown argument: ${args[index]}`);
  }
  if (!['mock', 'real-metrics', 'real-agent'].includes(mode)) throw new Error(`unsupported mode: ${mode}`);
  return { mode, full, confirmed };
}

function printUsage() {
  process.stdout.write(`usage: ./scripts/mtb [command]\n\ncommands:\n  up [--mode mock|real-metrics|real-agent]       build and keep a development stack running (default)\n  verify [--full] [--mode ...]                   run gates and an isolated E2E\n  e2e [--mode ...]                               run only an isolated E2E\n  ps|logs|down [--mode ...]                      manage the current development stack\n  reset --yes [--mode ...]                       remove the current development stack and volume\n  init --slot N [--name ID] [--force]            initialize this worktree\n  config show|check                              inspect worktree configuration\n  diagnose real-metrics|deepseek                 run a layered diagnostic\n  run assistant-mcp|ai-core                      run one service on worktree-local ports\n  doctor                                         validate the local toolchain\n  deps                                           install stale or absent frontend dependencies\n`);
}

export function main(args = process.argv.slice(2)) {
  const command = args.shift() ?? 'up';
  if (command === 'help' || command === '--help') {
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
  if (command === 'up') {
    const options = parseCommandOptions(args);
    runDevUp(loadConfig(), options.mode);
    return;
  }
  if (command === 'verify') {
    const options = parseCommandOptions(args, { allowFull: true });
    runVerification(loadConfig(), options.mode, options.full);
    return;
  }
  if (command === 'e2e') {
    const options = parseCommandOptions(args);
    runE2ECommand(loadConfig(), options.mode);
    return;
  }
  if (['ps', 'logs', 'down', 'reset'].includes(command)) {
    const options = parseCommandOptions(args, { allowYes: command === 'reset' });
    runDevLifecycle(loadConfig(), command, options.mode, options.confirmed);
    return;
  }
  if (command === 'diagnose') {
    const target = args.shift();
    if (args.length) throw new Error(`unknown diagnose argument: ${args[0]}`);
    runDiagnostic(loadConfig(), target);
    return;
  }
  if (command === 'run') {
    const service = args.shift();
    if (args.length) throw new Error(`unknown run argument: ${args[0]}`);
    runLocalService(loadConfig(), service);
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
