# Agent Memory & Guidelines

This file serves as a persistent memory and rulebook for AI agents working on this project. It should be updated whenever new patterns, constraints, or decisions are discovered.

## Technology Stack & Idioms

- **Go Version**: The project uses Go 1.25.5.
  - We can and should use the `any` keyword as an alias for `interface{}`.

## Project Structure & Patterns

- **API Responses**: Response structs are defined in `pkg/nomadtable/responses.go` rather than being inline or in `client.go`.
  - Nested objects must be pointers.
- **API Client**: The client implementation is in `pkg/nomadtable/client.go`.
