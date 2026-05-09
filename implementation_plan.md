# Implement ExcludedPOMs as []string in Librarian

We need to update `ExcludedPOMs` configuration from `string` to `[]string` to correctly list artifact IDs of POMs that should not be managed by librarian. These excluded POMs should be ignored when identifying missing POMs or collecting POMs for updates. We also need to update the migration script to parse comma-separated strings from `generation_config.yaml` into slices.

## User Review Required

> [!IMPORTANT]
> Changing `ExcludedPOMs` from `string` to `[]string` is a breaking change for the configuration format and `.repo-metadata.json` schema if tools consuming it expect a string. We assume the migration from `generation_config.yaml` justifies this change to a list format.

## Proposed Changes

### Configuration

#### [MODIFY] [language.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/internal/config/language.go)
- Change `ExcludedPOMs` field in `JavaModule` struct from `string` to `[]string`.

### Librarian Core

#### [MODIFY] [repometadata.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/internal/librarian/java/repometadata.go)
- Change `ExcludedPOMs` field in `repoMetadata` struct from `string` to `[]string` to match configuration.

#### [MODIFY] [library.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/internal/librarian/library.go)
- Update `mergeJava` function to correctly merge `ExcludedPOMs` as a slice (override if src is not nil).

#### [MODIFY] [library_test.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/internal/librarian/library_test.go)
- Update `TestMergeJava` to use slice for `ExcludedPOMs`.

#### [MODIFY] [pom.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/internal/librarian/java/pom.go)
- Implement exclusion logic. We propose filtering out excluded artifact IDs in `discoverModules` so that they are automatically ignored by both `IdentifyMissingModules` and `collectModules`.
- Helper function `isExcludedPOM(artifactID string, excluded []string) bool` will be added.

#### [MODIFY] [pom_test.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/internal/librarian/java/pom_test.go)
- Add test cases to `TestIdentifyMissingModules` and `TestCollectModules` to verify exclusion functionality, following user rules (no refactoring existing tests, use `test` loop variable).

### Migration Script

#### [MODIFY] [java.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/tool/cmd/migrate/java.go)
- Update `buildConfig` to parse `l.ExcludedPoms` (comma-separated string) into `[]string` before assigning to `lib.Java.ExcludedPOMs`.
- Add helper function `parseStringList(s string) []string`.

#### [MODIFY] [java_test.go](file:///usr/local/google/home/zhumin/repos/testjava/librarian/tool/cmd/migrate/java_test.go)
- Update `TestBuildConfig` to expect `[]string` for `ExcludedPOMs` in the expected struct.

## Verification Plan

### Automated Tests
- Run unit tests for affected packages using `go test`:
  ```bash
  go test ./internal/config/...
  go test ./internal/librarian/...
  go test ./tool/cmd/migrate/...
  ```
