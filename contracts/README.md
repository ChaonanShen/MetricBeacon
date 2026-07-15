# Contracts

This directory is the single source of truth for all cross-process wire shapes:
Plugin Resource OpenAPI, AI Core OpenAPI, shared JSON Schema, Task SSE events, MCP Tool
schemas and error codes. G1 creates and validates the actual sources; consumers must not
introduce local duplicate DTOs before then.

Operational Knowledge, Skill, Playbook, and alert-mapping files live under
`data/operational-assets`. Their parsed shapes are governed by `schemas/assets`; Markdown
assets use strict YAML frontmatter plus a `content` field supplied by the loader. The
contract validation gate checks both valid assets and negative capability/step fixtures.
