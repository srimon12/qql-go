from __future__ import annotations

import json
import os
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[4]
DEFAULT_QQL_BIN = REPO_ROOT / ("qql.exe" if os.name == "nt" else "qql")
QQL_BIN = os.environ.get("QQL_BIN", str(DEFAULT_QQL_BIN))


@dataclass
class Result:
    message: str
    data: Any = None
    operation: str | None = None


def execute_json(query: str) -> Result:
    try:
        completed = subprocess.run(
            [QQL_BIN, "exec", "--quiet", "--json", query],
            capture_output=True,
            text=True,
        )
    except FileNotFoundError as exc:
        raise RuntimeError(f"Unable to run qql binary at {QQL_BIN}") from exc

    stdout = completed.stdout.strip()
    stderr = completed.stderr.strip()

    if not stdout:
        raise RuntimeError(stderr or f"qql exited with code {completed.returncode}")

    try:
        payload = json.loads(stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"qql did not return valid JSON: {stdout}") from exc

    if completed.returncode != 0 or not payload.get("ok", False):
        raise RuntimeError(payload.get("error") or stderr or "qql command failed")

    return Result(
        message=payload.get("message", ""),
        data=payload.get("data"),
        operation=payload.get("operation"),
    )


def drop_collection_if_exists(name: str) -> None:
    try:
        execute_json(f"DROP COLLECTION {name}")
    except Exception as exc:
        message = str(exc).lower()
        if "does not exist" in message or "not found" in message:
            return
        raise


def print_result(label: str, result: Result, limit: int = 5) -> None:
    print(f"[{label}] {result.message}")
    data = result.data
    if isinstance(data, dict):
        results = data.get("results")
        if isinstance(results, list):
            for hit in results[:limit]:
                score = hit.get("score")
                hit_id = hit.get("id")
                print(f"  score={score} id={hit_id}")
        elif data:
            print(f"  {data}")
    elif isinstance(data, list):
        print(f"  {data}")
    print()
