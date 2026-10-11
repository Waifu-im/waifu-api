
---

## Parity work, 2026-10-12

The branch was brought back in step with `main` (which had gained WebP ingest
since the fork) and then audited for behavioural drift.

### What the audit found

Five defects, all of which the existing tests were structurally unable to see:

| Defect | Effect |
|---|---|
| `userSelect` subqueries had no `AS` aliases | `GET /users` returned 500 for every caller, admins included |
| `UserFilter.IncludedID` was a single id | a multi-select user picker returned one user instead of all of them |
| `ApiKeyDto` omitted `userId` | every client read `undefined` for a key's owner |
| `ApiKeyDto` tagged nullable timestamps `omitempty` | unset values dropped the key where C# emits `null` |
| `handleGetTag` / `handleGetTagBySlug` omitted `ReviewStatus` | a single-tag lookup returned `null` where the listing returned `Accepted` |
| `applyNewContent` ran with a nil transaction | `PATCH /review/tasks/{id}` panicked and returned 500 on every call |
| the same branch wrote a nil payload into `jsonb` | would have failed the write once the panic was fixed |

### Why the tests missed them

Three separate reasons, each of which needed a different kind of check:

- **Hand-written SQL has no compiler.** The unaliased subquery and the nil
  transaction are both syntactically valid Go. Only executing them against a
  real Postgres finds them, which is why the suite demands
  `WAIFU_TEST_DATABASE_URL` and refuses to skip.
- **DTO drift is invisible across languages.** Nothing links `ApiKeyDto.cs` to
  `apiKeyDto`. `dto_contract_test.go` now parses the C# classes and reflects over
  the Go structs, comparing camelCase field sets in both directions.
- **Gaps cluster in the untested paths.** `internal/authz` and
  `internal/cdncache` shipped with no tests at all, and `GET /users` sat at 7%.
  Both `GET /users` bugs lived in the gap.

### Deliberate non-differences

Two places where Go looks wrong but matches C# exactly, both now pinned by a
test so they are not "fixed" later:

- An empty string in an edit payload means *no change*, not *clear*. See
  `EditPreparer.cs`: `hasDescription := !string.IsNullOrEmpty(...)`.
- `CreatorId` and `ImageCount` are left unset on single-tag lookups because
  `GetTagById/Query.cs` does not populate them either.

### Standing advice

When comparing against the C# service, read the C# first. Twice during this
audit the apparent Go bug was the C# behaviour faithfully reproduced, and
"fixing" it would have introduced the divergence.
