import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import YAML from 'yaml';

const root = path.resolve(process.argv[2] ?? '.');
const contracts = path.join(root, 'contracts');
const examples = path.join(contracts, 'examples');
const scenario = path.join(root, 'data', 'mock-scenarios', 'node_exporter_overview');
const operationalAssets = path.join(root, 'data', 'operational-assets');
const ajv = new Ajv2020({ allErrors: true, strict: false });
addFormats(ajv);

async function json(file) {
  return JSON.parse(await readFile(file, 'utf8'));
}

async function jsonFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const descendants = await Promise.all(entries.map(async (entry) => {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) return jsonFiles(entryPath);
    return entry.name.endsWith('.json') ? [entryPath] : [];
  }));
  return descendants.flat();
}

function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exitCode = 1;
}

function validate(schemaId, value, label) {
  const validator = ajv.getSchema(schemaId);
  if (!validator) throw new Error(`schema is not registered: ${schemaId}`);
  if (!validator(value)) {
    fail(`${label} is invalid:\n${ajv.errorsText(validator.errors, { separator: '\n' })}`);
  }
}

function expectInvalid(schemaId, value, label) {
  const validator = ajv.getSchema(schemaId);
  if (!validator) throw new Error(`schema is not registered: ${schemaId}`);
  if (validator(value)) fail(`${label} unexpectedly passed validation`);
}

async function markdownAsset(relative) {
  const source = await readFile(path.join(operationalAssets, relative), 'utf8');
  const match = source.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n([\s\S]+)$/);
  if (!match) throw new Error(`${relative} must contain YAML frontmatter and Markdown content`);
  return {...YAML.parse(match[1]), content: match[2].trim()};
}

const schemaFiles = [
  ...(await jsonFiles(path.join(contracts, 'schemas'))),
  ...(await jsonFiles(path.join(contracts, 'events'))),
  ...(await jsonFiles(path.join(contracts, 'tools'))),
];

for (const file of schemaFiles) {
  const schema = await json(file);
  if (!schema.$id) throw new Error(`${path.relative(root, file)} has no $id`);
  ajv.addSchema(schema);
}
for (const file of schemaFiles) {
  ajv.getSchema((await json(file)).$id);
}

const id = (relative) => `https://mini-torchbearing.local/contracts/${relative}`;
validate(id('tools/grafana/search-metrics.output.schema.json'), await json(path.join(scenario, 'search_metrics.json')), 'search_metrics fixture');
const labels = await json(path.join(scenario, 'metric_labels.json'));
for (const [index, entry] of labels.labels.entries()) {
  validate(id('tools/grafana/get-metric-labels.output.schema.json'), entry, `metric_labels fixture entry ${index}`);
}
for (const name of ['query_cpu.json', 'query_memory.json', 'query_load.json']) {
  validate(id('tools/grafana/query-prometheus.output.schema.json'), await json(path.join(scenario, name)), `${name} fixture`);
}
validate(id('schemas/expected-task-events.schema.json'), await json(path.join(scenario, 'expected_task_events.json')), 'expected_task_events fixture');
validate(id('schemas/mock-scenario-manifest.schema.json'), YAML.parse(await readFile(path.join(scenario, 'manifest.yaml'), 'utf8')), 'scenario manifest');
validate(id('schemas/api/create-session-request.schema.json'), await json(path.join(examples, 'create-session.request.json')), 'create Session example');
validate(id('schemas/api/session-page.schema.json'), await json(path.join(examples, 'session-page.response.json')), 'Session page example');
validate(id('schemas/api/create-task-request.schema.json'), await json(path.join(examples, 'create-task.request.json')), 'create Task example');
validate(id('events/task-events.schema.json'), await json(path.join(examples, 'task-event.json')), 'TaskEvent example');
validate(id('tools/grafana/search-metrics.input.schema.json'), await json(path.join(examples, 'grafana.search_metrics.input.json')), 'search tool example');
validate(id('tools/grafana/get-metric-labels.input.schema.json'), await json(path.join(examples, 'grafana.get_metric_labels.input.json')), 'labels tool example');
validate(id('tools/grafana/query-prometheus.input.schema.json'), await json(path.join(examples, 'grafana.query_prometheus.input.json')), 'query tool example');
const knowledgeAsset = await markdownAsset('knowledge/order-service.v1.md');
const skillAsset = await markdownAsset('skills/diagnose-order-backlog.v1.md');
validate(id('schemas/assets/knowledge.schema.json'), knowledgeAsset, 'order service Knowledge');
validate(id('schemas/assets/skill.schema.json'), skillAsset, 'order backlog Skill');
expectInvalid(id('schemas/assets/knowledge.schema.json'), {...knowledgeAsset, healthyState: {...knowledgeAsset.healthyState, workerConcurrency: 0}}, 'Knowledge with fault state as healthy');
expectInvalid(id('schemas/assets/skill.schema.json'), {...skillAsset, allowedCapabilities: [...skillAsset.allowedCapabilities, 'fault.inject']}, 'Skill with a fault capability');
const playbookAsset =
  YAML.parse(await readFile(path.join(operationalAssets, 'playbooks', 'order-queue-backlog.v1.yaml'), 'utf8'));
validate(
  id('schemas/assets/playbook.schema.json'),
  playbookAsset,
  'order queue backlog Playbook',
);
expectInvalid(id('schemas/assets/playbook.schema.json'), {...playbookAsset, steps: [...playbookAsset.steps, 'run_shell']}, 'Playbook with an executable free-form step');
expectInvalid(id('schemas/assets/playbook.schema.json'), {...playbookAsset, allowedCapabilities: [...playbookAsset.allowedCapabilities, 'order_service.restore_worker_concurrency']}, 'Playbook exposing a write capability to diagnosis');
const alertMappingAsset =
  YAML.parse(await readFile(path.join(operationalAssets, 'mappings', 'order-demo.v1.yaml'), 'utf8'));
validate(
  id('schemas/assets/alert-mapping.schema.json'),
  alertMappingAsset,
  'order demo alert mapping',
);
expectInvalid(id('schemas/assets/alert-mapping.schema.json'), {...alertMappingAsset, mappings: alertMappingAsset.mappings.map(({requiredLabels: _requiredLabels, ...mapping}) => mapping)}, 'Alert mapping without required labels');

const errorSchema = ajv.getSchema(id('schemas/common/error.schema.json')).schema;
const documentedCodes = Object.keys(YAML.parse(await readFile(path.join(contracts, 'errors', 'error-codes.yaml'), 'utf8')).codes).sort();
const schemaCodes = [...errorSchema.$defs.error.properties.code.enum].sort();
if (JSON.stringify(documentedCodes) !== JSON.stringify(schemaCodes)) {
  fail('error-codes.yaml and error.schema.json enumerate different codes');
}

if (!process.exitCode) process.stdout.write(`validated ${schemaFiles.length} JSON Schemas and node_exporter_overview fixtures\n`);
