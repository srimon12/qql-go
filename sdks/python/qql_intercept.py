#!/usr/bin/env python3
"""
qql_intercept.py — Runtime interceptor for Qdrant Python SDK.

Wraps QdrantClient at the HTTP layer, captures every REST API call
as JSON, and converts it to QQL via the Go converter.

Usage:
    python3 qql_intercept.py your_script.py
    python3 qql_intercept.py your_script.py -o output.qql
"""

import sys
import os
import json
import shutil
import subprocess
from pathlib import Path
from urllib.parse import urlparse

captured = []


def intercept_request(original_request):
    """Patch ApiClient.request to capture REST calls."""

    def patched(self, *, type_, method, url, path_params=None, **kwargs):
        if path_params is None:
            path_params = {}

        # Build the actual URL to extract path
        host = self.host if self.host.endswith("/") else self.host + "/"
        url_clean = url[1:] if url.startswith("/") else url
        full_url = urljoin(host, url_clean.format(**path_params)) if 'urljoin' in dir() else url_clean.format(**path_params)

        # Extract path from URL
        parsed = urlparse(full_url)
        path = parsed.path

        # Extract body
        body = None
        if "content" in kwargs:
            try:
                body = json.loads(kwargs["content"])
            except (json.JSONDecodeError, TypeError):
                body = kwargs["content"]
        elif "json" in kwargs:
            body = kwargs["json"]

        captured.append({"method": method, "path": path, "body": body})
        print(f"  CAPTURED: {method} {path}", file=sys.stderr)

        # Return mock response so script doesn't crash
        return mock_response(type_)

    return patched


def mock_response(type_):
    """Return a mock httpx.Response."""
    from unittest.mock import MagicMock
    resp = MagicMock()
    resp.status_code = 200
    resp.json.return_value = {"status": "ok", "result": {"status": "green"}}
    resp.text = '{"status":"ok"}'
    resp.content = b'{"status":"ok"}'
    resp.headers = {"content-type": "application/json"}
    resp.ok = True
    return resp


def convert_to_qql(call):
    """Convert a captured REST call to QQL via qql-go convert."""
    wrapped = json.dumps({
        "method": call["method"],
        "path": call["path"],
        "body": call["body"]
    }, separators=(",", ":"))

    # Find qql-go binary
    qql_bin = Path(__file__).parent.parent.parent / "qql-go"
    if not qql_bin.exists():
        qql_bin = shutil.which("qql-go")

    if not qql_bin:
        print("Error: qql-go binary not found. Run 'go build -o qql-go ./cmd/qql-go' first.", file=sys.stderr)
        sys.exit(1)

    result = subprocess.run(
        [str(qql_bin), "convert", "--quiet"],
        input=wrapped, capture_output=True, text=True
    )
    if result.returncode == 0 and result.stdout.strip():
        return result.stdout.strip()
    else:
        print(f"Convert error: {result.stderr.strip()}", file=sys.stderr)
        return f"-- CONVERT ERROR: {result.stderr.strip()}"



def main():
    import argparse
    parser = argparse.ArgumentParser(description="Convert Qdrant Python SDK calls to QQL")
    parser.add_argument("script", help="Python script using qdrant_client")
    parser.add_argument("-o", "--output", help="Output file (default: stdout)")
    args = parser.parse_args()

    if not os.path.exists(args.script):
        print(f"Error: {args.script} not found", file=sys.stderr)
        sys.exit(1)

    # Read the script
    with open(args.script) as f:
        source = f.read()

    # Patch QdrantClient HTTP layer
    try:
        from qdrant_client.http.api_client import ApiClient
        original = ApiClient.request
        ApiClient.request = intercept_request(original)
    except ImportError:
        print("Error: qdrant_client not installed. Run: uv pip install qdrant-client", file=sys.stderr)
        sys.exit(1)

    # Execute the script
    try:
        exec(compile(source, args.script, "exec"), {"__name__": "__main__", "__file__": args.script})
    except SystemExit:
        pass
    except Exception as e:
        print(f"Script error (expected): {e}", file=sys.stderr)

    # Restore original
    ApiClient.request = original

    if not captured:
        print("No API calls captured.", file=sys.stderr)
        sys.exit(1)

    print(f"\nCaptured {len(captured)} API calls\n", file=sys.stderr)

    # Convert to QQL
    qql_lines = []
    for call in captured:
        qql = convert_to_qql(call)
        qql_lines.append(qql)

    output = "\n\n".join(qql_lines) + "\n"

    if args.output:
        with open(args.output, "w") as f:
            f.write(output)
        print(f"Written to {args.output}", file=sys.stderr)
    else:
        print(output)


if __name__ == "__main__":
    main()
