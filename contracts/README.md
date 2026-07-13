# Contracts

This directory is the single source of truth for all cross-process wire shapes:
Plugin Resource OpenAPI, AI Core OpenAPI, shared JSON Schema, Task SSE events, MCP Tool
schemas and error codes. G1 creates and validates the actual sources; consumers must not
introduce local duplicate DTOs before then.
