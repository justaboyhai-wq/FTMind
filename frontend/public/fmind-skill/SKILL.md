---
name: fmind
description: >
  Import documents and retrieve knowledge through the FMind REST API. Use
  for uploading files, URLs, or Markdown to a knowledge base; hybrid search
  within a knowledge base; cross-knowledge-base search; and browsing imported
  knowledge. Requires FMIND_BASE_URL and FMIND_API_KEY.
metadata:
  openclaw:
    requires:
      env: [FMIND_BASE_URL, FMIND_API_KEY]
---

# FMind knowledge base

Use the FMind REST API to import content and retrieve grounded context from
the user's knowledge bases. Never expose the API key in messages, commands
logged to shared output, or saved files.

## Setup

1. In FMind, open **Settings → API Integration** and create or copy an API
   key.
2. Configure the agent environment with the public FMind API address. The
   address must end with `/api/v1` and be reachable from the agent runtime.

```bash
export FMIND_BASE_URL="https://fmind.example.com/api/v1"
export FMIND_API_KEY="sk-your-api-key"
```

For a local deployment used by an agent on the same computer, the usual base
URL is `http://localhost:8080/api/v1`.

## Credential check

Before making an API request, ensure both values exist. If either is missing,
ask the user to configure it; do not guess or substitute a token.

```bash
if [ -z "$FMIND_BASE_URL" ] || [ -z "$FMIND_API_KEY" ]; then
  echo "Missing FMind credentials. Set FMIND_BASE_URL and FMIND_API_KEY."
  exit 1
fi
```

## Request helper

All JSON API requests use `X-API-Key`. Keep the endpoint relative to the base
URL so deployments behind a reverse proxy continue to work.

```bash
fmind_api() {
  local method="$1" endpoint="$2" body="$3"
  curl --fail-with-body -sS -X "$method" "$FMIND_BASE_URL/$endpoint" \
    -H "X-API-Key: $FMIND_API_KEY" \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: $(uuidgen 2>/dev/null || date +%s)" \
    ${body:+-d "$body"}
}
```

For uploads, use `curl -F` directly. Do not set `Content-Type` manually for a
multipart request.

## Choose the right API

| User intent | Endpoint | Notes |
| --- | --- | --- |
| List knowledge bases | `GET /knowledge-bases` | Select a KB by `id` or name before importing/searching. |
| View KB details | `GET /knowledge-bases/:id` | Inspect indexing and configuration. |
| Upload a file | `POST /knowledge-bases/:id/knowledge/file` | Multipart field: `file`; optional `enable_multimodel`. |
| Import a web page | `POST /knowledge-bases/:id/knowledge/url` | JSON: `url`, optional `enable_multimodel`. |
| Create Markdown knowledge | `POST /knowledge-bases/:id/knowledge/manual` | JSON: `title`, `content`, optional `tag_id`. |
| Check processing | `GET /knowledge/:id` | Poll `parse_status` after an import. |
| Browse KB entries | `GET /knowledge-bases/:id/knowledge` | Use `page`, `page_size`, optional `tag_id`. |
| Edit Markdown knowledge | `PUT /knowledge/manual/:id` | JSON: `title`, `content`. |
| Delete a knowledge entry | `DELETE /knowledge/:id` | Confirm destructive actions with the user first. |
| Search one KB | `GET /knowledge-bases/:id/hybrid-search` | JSON body: `query_text`, `match_count`, thresholds. |
| Search several KBs | `POST /knowledge-search` | JSON: `query`, `knowledge_base_ids`. |

## Common workflows

### Upload a file and wait for parsing

```bash
# First find the target knowledge base and its id.
fmind_api GET "knowledge-bases"

# Upload. The response contains data.id (knowledge id).
curl --fail-with-body -sS -X POST "$FMIND_BASE_URL/knowledge-bases/<kb_id>/knowledge/file" \
  -H "X-API-Key: $FMIND_API_KEY" \
  -F 'file=@document.pdf' \
  -F 'enable_multimodel=true'

# Poll until data.parse_status is completed or failed.
fmind_api GET "knowledge/<knowledge_id>"
```

### Import a URL or Markdown

```bash
fmind_api POST "knowledge-bases/<kb_id>/knowledge/url" \
  '{"url":"https://example.com/article","enable_multimodel":true}'

fmind_api POST "knowledge-bases/<kb_id>/knowledge/manual" \
  '{"title":"Meeting notes","content":"# Q1 review\n\nKey points..."}'
```

### Retrieve knowledge

```bash
# Hybrid retrieval within one knowledge base. This GET endpoint expects a JSON body.
fmind_api GET "knowledge-bases/<kb_id>/hybrid-search" \
  '{"query_text":"deployment process","match_count":5}'

# Search across selected knowledge bases.
fmind_api POST "knowledge-search" \
  '{"query":"deployment process","knowledge_base_ids":["kb-1","kb-2"]}'
```

## Response handling

- Successful responses put data in `data` (lists are usually `data[]`).
- Knowledge processing progresses from `pending` to `processing`, then
  `completed` or `failed`. Check `error_message` before retrying a failure.
- Hybrid-search results include `content`, `score`, `knowledge_id`,
  `knowledge_title`, and chunk metadata. Use them as source context and say
  when no relevant result was returned.
- Paginated knowledge lists return `total`, `page`, and `page_size`.

## Safety and error handling

- Do not upload, import, overwrite, or delete anything without the user's
  explicit target knowledge base and confirmation for destructive actions.
- Treat `401` as invalid/missing API credentials, `403` as insufficient access,
  `404` as a wrong resource id, `413` as an oversized upload, and `429` as a
  rate limit requiring a pause before retry.
- For a failed parse, inspect `error_message`; retry with
  `POST /knowledge/:id/reparse` only when the user asks to retry.
