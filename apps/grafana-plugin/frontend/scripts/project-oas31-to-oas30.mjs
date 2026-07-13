import { readFile, writeFile } from 'node:fs/promises';
import process from 'node:process';
import YAML from 'yaml';

const [input, output] = process.argv.slice(2);
if (!input || !output) throw new Error('usage: project-oas31-to-oas30.mjs <input> <output>');

function project(value) {
  if (Array.isArray(value)) return value.map(project);
  if (value === null || typeof value !== 'object') return value;

  const result = Object.fromEntries(Object.entries(value).map(([key, child]) => [key, project(child)]));
  delete result.$schema;
  delete result.$id;
  delete result.$defs;

  if (Object.hasOwn(result, 'const')) {
    result.enum = [result.const];
    delete result.const;
  }

  if (Array.isArray(result.anyOf)) {
    const nonNull = result.anyOf.filter((schema) => !(schema && schema.type === 'null'));
    if (nonNull.length === 1 && nonNull.length !== result.anyOf.length) {
      delete result.anyOf;
      Object.assign(result, nonNull[0], { nullable: true });
    }
  }
  return result;
}

const document = project(YAML.parse(await readFile(input, 'utf8')));
document.openapi = '3.0.3';
await writeFile(output, YAML.stringify(document), 'utf8');
