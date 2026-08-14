---
name: id-ledger-query
description: Query collected IDs from ID Ledger by project and whole-second RFC3339 time range, automatically following cursor pagination. Use when an agent needs to retrieve, inspect, or export IDs collected for a project during a time window.
---

# Query ID Ledger

Set `ID_LEDGER_URL` and `ID_LEDGER_TOKEN`, then run the bundled script using the path to this Skill directory:

```sh
python3 <skill-dir>/scripts/query_ids.py PROJECT_KEY \
  --since 2026-08-14T00:00:00Z \
  --until 2026-08-15T00:00:00Z
```

Use whole-second RFC3339 bounds. Omit `--until` to use the server's current-time bound, or set `--limit` between 1 and 1000 for the page size.

Read one JSON object per output line. The script follows `next_cursor` until exhausted and exits non-zero with an error on authentication, HTTP, network, or malformed-response failures. Never print or pass the token on the command line.
