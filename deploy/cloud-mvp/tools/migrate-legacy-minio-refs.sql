-- Rewrite legacy MinIO URLs after their corresponding logical objects have
-- been copied to OSS. Run with psql variables, for example:
--
--   docker compose exec -T postgres psql -U fmind -d fmind \
--     -v old_prefix='minio://blexwiki/' \
--     -v new_prefix='oss://keystore001/fmind-mvp/legacy-import-YYYYMMDD/blexwiki/' \
--     -f /path/to/migrate-legacy-minio-refs.sql
--
-- Take a database backup first. This script deliberately does not issue a
-- COMMIT: inspect the counts and then type COMMIT in the same psql session, or
-- append COMMIT only after a successful object-level verification.

\if :{?old_prefix}
\else
  \quit 3
\endif
\if :{?new_prefix}
\else
  \quit 3
\endif

BEGIN;

UPDATE knowledges
SET file_path = replace(file_path, :'old_prefix', :'new_prefix'),
    updated_at = NOW()
WHERE file_path LIKE :'old_prefix' || '%';

UPDATE chunks
SET content = replace(content, :'old_prefix', :'new_prefix')
WHERE content LIKE '%' || :'old_prefix' || '%';

UPDATE chunks
SET image_info = replace(image_info, :'old_prefix', :'new_prefix')
WHERE image_info LIKE '%' || :'old_prefix' || '%';

UPDATE wiki_pages
SET content = replace(content, :'old_prefix', :'new_prefix'),
    updated_at = NOW()
WHERE content LIKE '%' || :'old_prefix' || '%';

UPDATE messages
SET content = replace(content, :'old_prefix', :'new_prefix'),
    updated_at = NOW()
WHERE content LIKE '%' || :'old_prefix' || '%';

SELECT 'knowledges.file_path' AS location, count(*) AS remaining
FROM knowledges WHERE file_path LIKE :'old_prefix' || '%'
UNION ALL SELECT 'chunks.content', count(*) FROM chunks WHERE content LIKE '%' || :'old_prefix' || '%'
UNION ALL SELECT 'chunks.image_info', count(*) FROM chunks WHERE image_info LIKE '%' || :'old_prefix' || '%'
UNION ALL SELECT 'wiki_pages.content', count(*) FROM wiki_pages WHERE content LIKE '%' || :'old_prefix' || '%'
UNION ALL SELECT 'messages.content', count(*) FROM messages WHERE content LIKE '%' || :'old_prefix' || '%';

-- COMMIT; -- intentionally disabled; verify first.
