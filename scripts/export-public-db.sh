#!/usr/bin/env bash
set -euo pipefail

# Run from the directory containing compose.yaml. The first argument is the
# database service name; the second is the output filename. The dump contains
# schema and approved public content only. Site-user tables have no rows, and
# nullable creator/uploader IDs are replaced with NULL.
db_service=${1:-db}
out=${2:-waifu-public.sql}

if [[ -e "$out" ]]; then
  printf 'Refusing to overwrite %s\n' "$out" >&2
  exit 1
fi
if ! docker compose config --services | grep -Fx "$db_service" >/dev/null; then
  printf 'Compose service %s was not found here. Run: docker compose config --services\n' "$db_service" >&2
  exit 1
fi

umask 077
tmp=$(mktemp "${out}.XXXXXX")
trap 'rm -f "$tmp"' EXIT

{
  printf 'Exporting schema...\n' >&2
  docker compose exec -T "$db_service" sh -c 'exec pg_dump --schema-only --no-owner --no-privileges --no-comments -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
  printf 'Exporting approved content...\n' >&2
  docker compose exec -T "$db_service" sh -c 'exec psql -X -q -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET client_encoding TO 'UTF8';
SELECT 'BEGIN;';
SELECT $$COPY public."Artists" ("Id","Name","Patreon","Pixiv","Twitter","DeviantArt","ReviewStatus","CreatorId") FROM stdin;$$;
\copy (SELECT "Id","Name","Patreon","Pixiv","Twitter","DeviantArt","ReviewStatus",NULL::bigint FROM public."Artists" WHERE "ReviewStatus" = 1 ORDER BY "Id") TO stdout
SELECT E'\\.';
SELECT $$COPY public."Tags" ("Id","Name","Slug","Description","ReviewStatus","CreatorId") FROM stdin;$$;
\copy (SELECT "Id","Name","Slug","Description","ReviewStatus",NULL::bigint FROM public."Tags" WHERE "ReviewStatus" = 1 ORDER BY "Id") TO stdout
SELECT E'\\.';
SELECT $$COPY public."Images" ("Id","PerceptualHash","Extension","DominantColor","Source","UploaderId","UploadedAt","IsNsfw","IsAnimated","Width","Height","ByteSize","ReviewStatus") FROM stdin;$$;
\copy (SELECT "Id","PerceptualHash","Extension","DominantColor","Source",NULL::bigint,"UploadedAt","IsNsfw","IsAnimated","Width","Height","ByteSize","ReviewStatus" FROM public."Images" WHERE "ReviewStatus" = 1 ORDER BY "Id") TO stdout
SELECT E'\\.';
SELECT $$COPY public."ArtistImage" ("ArtistsId","ImagesId") FROM stdin;$$;
\copy (SELECT ai."ArtistsId",ai."ImagesId" FROM public."ArtistImage" ai JOIN public."Artists" a ON a."Id"=ai."ArtistsId" AND a."ReviewStatus"=1 JOIN public."Images" i ON i."Id"=ai."ImagesId" AND i."ReviewStatus"=1 ORDER BY ai."ArtistsId",ai."ImagesId") TO stdout
SELECT E'\\.';
SELECT $$COPY public."ImageTag" ("ImagesId","TagsId") FROM stdin;$$;
\copy (SELECT it."ImagesId",it."TagsId" FROM public."ImageTag" it JOIN public."Images" i ON i."Id"=it."ImagesId" AND i."ReviewStatus"=1 JOIN public."Tags" t ON t."Id"=it."TagsId" AND t."ReviewStatus"=1 ORDER BY it."ImagesId",it."TagsId") TO stdout
SELECT E'\\.';
SELECT $$COPY public."__EFMigrationsHistory" ("MigrationId","ProductVersion") FROM stdin;$$;
\copy (SELECT "MigrationId","ProductVersion" FROM public."__EFMigrationsHistory" ORDER BY "MigrationId") TO stdout
SELECT E'\\.';
SELECT $$SELECT pg_catalog.setval(pg_get_serial_sequence('public."Artists"','Id'),COALESCE((SELECT MAX("Id") FROM public."Artists"),1),EXISTS(SELECT 1 FROM public."Artists"));$$;
SELECT $$SELECT pg_catalog.setval(pg_get_serial_sequence('public."Tags"','Id'),COALESCE((SELECT MAX("Id") FROM public."Tags"),1),EXISTS(SELECT 1 FROM public."Tags"));$$;
SELECT $$SELECT pg_catalog.setval(pg_get_serial_sequence('public."Images"','Id'),COALESCE((SELECT MAX("Id") FROM public."Images"),1),EXISTS(SELECT 1 FROM public."Images"));$$;
SELECT 'COMMIT;';
COMMIT;
SQL
} > "$tmp"
test -s "$tmp"
mv "$tmp" "$out"
trap - EXIT
printf 'Created %s\n' "$out"
