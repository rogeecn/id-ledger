#!/usr/bin/env python3
"""Query all ID Ledger pages and emit one JSON record per line."""

import argparse
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen


class QueryError(RuntimeError):
    """A clear, user-facing query failure."""


def query_ids(base_url, token, project, since, until=None, limit=100, opener=urlopen):
    cursor = None
    seen_cursors = set()
    endpoint = f"{base_url.rstrip('/')}/v1/projects/{quote(project, safe='')}/ids"

    while True:
        params = {"limit": limit}
        if cursor:
            params["cursor"] = cursor
        else:
            params["since"] = since
            if until:
                params["until"] = until

        try:
            request = Request(
                f"{endpoint}?{urlencode(params)}",
                headers={"Accept": "application/json", "Authorization": f"Bearer {token}"},
            )
            with opener(request, timeout=30) as response:
                payload = json.load(response)
        except HTTPError as error:
            body = error.read().decode("utf-8", "replace")
            try:
                payload = json.loads(body)
                detail = payload.get("error", body) if isinstance(payload, dict) else body
            except json.JSONDecodeError:
                detail = body
            raise QueryError(f"HTTP {error.code}: {str(detail).strip() or error.reason}") from error
        except URLError as error:
            raise QueryError(f"request failed: {error.reason}") from error
        except json.JSONDecodeError as error:
            raise QueryError(f"invalid response: {error}") from error
        except ValueError as error:
            raise QueryError(f"invalid URL: {error}") from error
        except OSError as error:
            raise QueryError(f"invalid response: {error}") from error

        if not isinstance(payload, dict) or not isinstance(payload.get("items"), list):
            raise QueryError("invalid response: expected an items array")
        for item in payload["items"]:
            if not isinstance(item, dict) or not isinstance(item.get("id"), str) or not isinstance(item.get("created_at"), str):
                raise QueryError("invalid response: malformed item")
            yield item

        cursor = payload.get("next_cursor")
        if cursor is None or cursor == "":
            return
        if not isinstance(cursor, str):
            raise QueryError("invalid response: malformed next_cursor")
        if cursor in seen_cursors:
            raise QueryError("invalid response: repeated next_cursor")
        seen_cursors.add(cursor)


def page_size(value):
    parsed = int(value)
    if not 1 <= parsed <= 1000:
        raise argparse.ArgumentTypeError("must be between 1 and 1000")
    return parsed


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("project", help="stable project key")
    parser.add_argument("--since", required=True, help="inclusive whole-second RFC3339 bound")
    parser.add_argument("--until", help="exclusive whole-second RFC3339 bound")
    parser.add_argument("--limit", type=page_size, default=100, help="page size (default: 100)")
    parser.add_argument("--base-url", default=os.environ.get("ID_LEDGER_URL"))
    args = parser.parse_args(argv)

    token = os.environ.get("ID_LEDGER_TOKEN")
    if not args.base_url:
        parser.error("set ID_LEDGER_URL or pass --base-url")
    if not token:
        parser.error("set ID_LEDGER_TOKEN")

    try:
        for item in query_ids(args.base_url, token, args.project, args.since, args.until, args.limit):
            print(json.dumps(item, ensure_ascii=False, separators=(",", ":")))
    except QueryError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
