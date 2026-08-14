import io
import json
import os
import unittest
from contextlib import redirect_stderr
from unittest.mock import patch
from urllib.error import HTTPError
from urllib.parse import parse_qs, urlparse

from query_ids import QueryError, main, query_ids


class FakeOpener:
    def __init__(self, *responses):
        self.responses = list(responses)
        self.requests = []

    def __call__(self, request, timeout):
        self.requests.append(request)
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return io.BytesIO(json.dumps(response).encode())


class QueryIDsTest(unittest.TestCase):
    def test_bearer_and_cursor_pagination(self):
        opener = FakeOpener(
            {"items": [{"id": "a", "created_at": "2026-08-14T00:00:00Z"}], "next_cursor": "next"},
            {"items": [{"id": "b", "created_at": "2026-08-13T23:59:59Z"}]},
        )

        items = list(
            query_ids(
                "https://ledger.example/",
                "secret",
                "team/A",
                "2026-08-13T00:00:00Z",
                "2026-08-15T00:00:00Z",
                2,
                opener,
            )
        )

        self.assertEqual([item["id"] for item in items], ["a", "b"])
        self.assertEqual(len(opener.requests), 2)
        self.assertEqual(opener.requests[0].get_header("Authorization"), "Bearer secret")
        self.assertEqual(urlparse(opener.requests[0].full_url).path, "/v1/projects/team%2FA/ids")
        self.assertEqual(
            parse_qs(urlparse(opener.requests[0].full_url).query),
            {"since": ["2026-08-13T00:00:00Z"], "until": ["2026-08-15T00:00:00Z"], "limit": ["2"]},
        )
        self.assertEqual(parse_qs(urlparse(opener.requests[1].full_url).query), {"cursor": ["next"], "limit": ["2"]})

    def test_http_failure_is_explicit(self):
        error = HTTPError(
            "https://ledger.example",
            401,
            "Unauthorized",
            None,
            io.BytesIO(b'{"error":"unauthorized"}'),
        )

        with self.assertRaisesRegex(QueryError, "HTTP 401: unauthorized"):
            list(query_ids("https://ledger.example", "bad", "demo", "2026-08-14T00:00:00Z", opener=FakeOpener(error)))

    def test_invalid_base_url_exits_without_traceback(self):
        stderr = io.StringIO()
        with patch.dict(os.environ, {"ID_LEDGER_TOKEN": "secret"}), redirect_stderr(stderr):
            status = main(["demo", "--since", "2026-08-14T00:00:00Z", "--base-url", "not-a-url"])

        self.assertEqual(status, 1)
        self.assertTrue(stderr.getvalue().startswith("error: invalid URL:"))
        self.assertNotIn("Traceback", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
