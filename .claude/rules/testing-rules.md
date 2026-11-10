# Testing, Documentation, and Accessibility Rules (PART 28, 29, 30)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never run Go builds, tests, or linters on the host.
- Never skip required container/OS integration scripts or browser-flow coverage.
- Never ship untranslated user-visible text or claim docs build without checking.
- Never treat the browser test suite as part of the commit gate.

## CRITICAL - ALWAYS DO

- Provide unit, integration, E2E, Docker, and Incus scripts required by PART 28.
- Keep tests isolated and clean every task-created container, VM, volume, and network.
- Build MkDocs and ReadTheDocs documentation from the canonical docs tree.
- Translate server and CLI strings with matching keys in all supported languages.
- Meet accessibility behavior and translation parity for both binaries.

## Key rules

- Run primary-session test/lint commands through the test-lint wrapper.
- Keep documentation, API schemas, and actual route behavior in sync.
- E2E browser coverage is on demand, not part of the commit gate.

For complete details, see AI.md PART 28, 29, 30
