# LSP & Semantic Code Tools

Two complementary tool sets provide semantic code intelligence. Both are
available simultaneously and should be preferred over grep/read for any
operation involving symbol relationships, cross-file impact, or refactoring.

## 1) Built-in LSP tool

Claude Code's native `LSP` tool talks to language servers on PATH. It
provides navigation-oriented operations:

| Operation              | What it does                                      |
|------------------------|---------------------------------------------------|
| `goToDefinition`       | Jump to a symbol's declaration                    |
| `findReferences`       | All usages of a symbol across the codebase        |
| `goToImplementation`   | Find all implementations of an interface          |
| `hover`                | Type info and documentation for a symbol          |
| `documentSymbol`       | All symbols in a file                             |
| `workspaceSymbol`      | Search symbols across the workspace               |
| `prepareCallHierarchy` | Get call hierarchy item at a position             |
| `incomingCalls`        | All functions/methods that call a given function  |
| `outgoingCalls`        | All functions/methods called by a given function  |

**Supported languages:**
- Go (via gopls) — fully functional
- TypeScript/JavaScript (via typescript-language-server) — requires plugin
  support; may not be available in all environments (see Limitations below)

## 2) gopls MCP tools (Go backend only)

gopls ships a built-in MCP server that provides higher-level semantic
operations beyond what the LSP tool offers:

| Tool                     | What it does                                          |
|--------------------------|-------------------------------------------------------|
| `go_workspace`           | Understand workspace layout (module, workspace, GOPATH) |
| `go_search`              | Fuzzy workspace symbol search by name                 |
| `go_file_context`        | File contents + intra-package dependency summary      |
| `go_package_api`         | Full exported API of a package                        |
| `go_symbol_references`   | All references to a symbol (by file + symbol name)    |
| `go_diagnostics`         | Compiler errors and vet warnings for edited files     |
| `go_vulncheck`           | Security vulnerability check on dependencies          |

These are configured as an MCP server in `.mcp.json` (via `gopls mcp`
over stdio) and available as `mcp__gopls__*` tools.

## 3) When to use which

| Task                          | Preferred tool                               |
|-------------------------------|----------------------------------------------|
| Navigate to a definition      | `LSP(goToDefinition)`                        |
| Find all usages of a symbol   | `LSP(findReferences)` or `go_symbol_references` |
| Find interface implementors   | `LSP(goToImplementation)`                    |
| Understand call flow          | `LSP(incomingCalls)` / `LSP(outgoingCalls)`  |
| Understand a file's deps      | `go_file_context`                            |
| Check for compile errors      | `go_diagnostics`                             |
| Explore a package's API       | `go_package_api`                             |
| Search for a symbol by name   | `LSP(workspaceSymbol)` or `go_search`        |
| Check dependency vulns        | `go_vulncheck`                               |

## 4) Mandatory usage

1. **Before modifying an interface** — run `LSP(findReferences)` or
   `go_symbol_references` on every method being changed to know all
   implementation sites before editing.

2. **After editing Go code** — run `go_diagnostics` on the edited
   files to catch breaks before the full `make test-backend` run.

3. **Exploring unfamiliar code** — use `LSP(workspaceSymbol)` or
   `go_search` to find a symbol, then `LSP(findReferences)` to
   understand its usage, rather than reading files top-down. Use
   `go_file_context` after reading any Go file for the first time
   to understand its intra-package dependencies.

4. **Adding a method to a store/service interface** — use
   `LSP(goToImplementation)` on the interface type to find all
   implementors and mock files that need updating.

5. **Adding or updating dependencies** — run `go_vulncheck` after
   modifying go.mod to check for known vulnerabilities.

## 5) Limitations

- gopls does not index `/backend/internal/api/gen` — those files are
  generated and must never be hand-edited.
- LSP tools require the Go module to be compilable. If codegen is stale
  (after a Goa DSL edit), run `make gen` before using LSP tools.
- TypeScript LSP availability depends on the Claude Code plugin loader.
  If `LSP` returns "No LSP server available for file type: .ts", fall
  back to grep/read for frontend symbol search.
