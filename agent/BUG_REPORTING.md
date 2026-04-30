# BUG_REPORTING.md — Bug report quality guide

This document defines the minimum quality bar for bug reports filed by agents. It applies to:
- QA runtime error sweep findings (filed as B-NNN tickets in backlog.yaml)
- Bug tickets filed during any phase of the development lifecycle

## Required fields

Every bug report must include:

### 1. Title
A brief, specific title suitable for a backlog ticket. Include the symptom and the component.
- Good: "FileSystem.OpenFile logs error-level for expected file-not-found in sidecar lookup"
- Bad: "Logging issue"

### 2. Call site context
Include the code path where the bug manifests:
- The function or method name (e.g., `FileSystem.OpenFile()`)
- The file path (e.g., `backend/internal/store/filesystem.go:142`)
- The caller context (e.g., "called from `image_metadata.go` sidecar lookup")

This helps the developer immediately understand scope without re-investigating.

### 3. Log evidence
The actual error log line(s) that triggered the finding. Quote them verbatim:
```
level=error msg="failed to open file" path="/data/samples/img001.json" error="open: no such file or directory"
```

### 4. Root cause hypothesis
1-2 sentences explaining the likely root cause:
- What condition triggers the bug
- Why the current behavior is incorrect
- What the correct behavior should be

Example: "The `OpenFile` method uses `logrus.Error` for all open failures, but file-not-found is expected when checking for optional sidecar files. Should use `logrus.Debug` for `os.ErrNotExist`."

### 5. Suggested acceptance criteria
1-3 concrete, testable criteria:
- "FileSystem.OpenFile logs at debug level (not error) when the error is os.ErrNotExist"
- "Error-level logging is preserved for unexpected filesystem errors (permission denied, I/O error)"

### 6. Suggested testing
Commands to verify the fix:
- "command: make test-backend"
- "command: make test-e2e"

### 7. Priority
A numeric priority (default: 70). Higher = more important. Guidelines:
- 90+: Crash, data loss, security vulnerability
- 70-89: Incorrect behavior visible to users
- 50-69: Incorrect behavior not visible to users (logging, internal state)
- 30-49: Code quality, minor inconsistency
- <30: Nice-to-have, cosmetic

## Example bug report (for backlog.yaml)

```yaml
- id: B-028
  title: "FileSystem.OpenFile logs error-level for expected file-not-found in sidecar lookup"
  priority: 55
  status: todo
  requires: []
  acceptance:
    - "FileSystem.OpenFile logs at debug level when error is os.ErrNotExist"
    - "Error-level logging preserved for unexpected filesystem errors"
  testing:
    - "command: make test-backend"
  notes: |
    Call site: FileSystem.OpenFile() in backend/internal/store/filesystem.go:142,
    called from image_metadata.go sidecar lookup.
    Log evidence: level=error msg="failed to open file" path="/data/samples/img001.json"
    Root cause: OpenFile uses logrus.Error for all open failures, but file-not-found
    is expected when checking for optional sidecar files.
```
