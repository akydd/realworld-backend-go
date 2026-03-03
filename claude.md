# Claude.md

This file contains project-local general instructions for Claude Code.

## Project overview
This repo is the backend api for a social blogging site, similar to Medium.com.

## Architecture
Refer to file `arch.md`.

## Development guidelines
* Always consult arch.md for up-to-date ppoject information.
* When writing a plan file for a feature file, write the plan to features/plans/{feature-name}-plan.md, where feature-name is the prefix of the corresponding feature file.
* When implementing a plan, if source code changes are needed, iterate on the code changes until there are no linter errors. Check for linter errors by running `make lint`.
* As the last step in implementing a plan, update the file arch.md as needed.
