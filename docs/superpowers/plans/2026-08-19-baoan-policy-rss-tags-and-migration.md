# Baoan policy filename, official tags, and RSS sync implementation plan

**Goal:** Preserve the current knowledge base while renaming policy documents, binding verified Baoan official tags, and making future RSS syncs update documents and tags without duplicates.

**Architecture:** The collector derives only website-backed, dimension-prefixed tags and emits them as RSS `<category>` values. FTMind's RSS connector carries those values through `FetchedItem` to the datasource service, which creates/reuses tags and applies managed official-tag relations while preserving manual tags. A transactional, idempotent migration adopts existing `post_<id>` documents and updates display names/tags without changing knowledge IDs, chunks, vectors, or storage paths.

## Requirements

- Stable business identity is `post_<id>`; RSS GUID is `baoan-policy:post_<id>` and never contains a snapshot ID.
- Exported names are `Official title（post_<id>）.md`, sanitized and truncated only at a UTF-8 boundary.
- Official tag namespaces are `服务对象/`, `发文机构/`, `主题/`, `文件载体/`, `文件类型/`, `关联内容/`, and conditionally `申报状态/当前可申报`.
- Tag values must come from website fields/dictionaries; no model-inferred official tags.
- “当前可申报” is assigned only when explicit start/end dates contain the current time; otherwise no application-status tag is created.
- Existing knowledge IDs, chunks, vectors, wiki bodies, and physical storage paths remain unchanged.
- Existing OR tag-filter semantics remain unchanged.

## Implementation tasks

1. Collector: add deterministic `OfficialTags`, website-value validation, application-date evaluation, RSS `<category>` output, stable GUIDs, and category-aware feed fingerprints.
2. FTMind RSS connector: add `FetchedItem.TagNames`, parse/validate categories, preserve fallback article fetching, and report invalid per-item categories as partial failures.
3. Datasource service: create/reuse tags, apply managed official tags alongside the source tag, preserve manual/AI tags, and remove stale managed tags on update.
4. Migration command: support `--kb-id`, `--feed-url`, `--data-dir`, `--dry-run`, `--apply`, `--rollback-file`, `--rollback`, and optional `--datasource-id`; map `post_<id>` documents to canonical packages; transactionally update title/file_name, cached wiki display titles, metadata, tag relations, and adoption cursor; abort on unmatched/ambiguous mappings.
5. Rollout: deploy collector first, deploy FTMind connector second, back up database/config, dry-run migration, apply migration and tag backfill, create paused RSS source, seed existing cursor/adoption metadata, then enable sync.

## Verification

- Unit tests cover official tag derivation/validation, date boundary behavior, stable GUIDs, category fingerprints, category parsing, tag reuse, stale-tag removal, and manual-tag preservation.
- Migration tests cover dry-run immutability, title mapping, illegal/long filename handling, ID/chunk/vector invariance, wiki references, idempotency, and rollback.
- Integration checks prove the existing documents are renamed, the tag manager is populated, initial RSS sync creates no duplicates, new policies receive titles and tags automatically, and 881 discovered IDs are accounted for with success/failure/missing output.

## Rollback and constraints

- Keep an original-name rollback file and database backup; pause RSS and restore names/tags/metadata on failure.
- Do not clear the current knowledge base, delete existing documents, alter unrelated ports/services, or infer website tags.
- The collector may continue its 881-policy backfill during migration; all commands must operate on the actual current count and remain idempotent.
