import json
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

from .gen.qql_pb2 import (
    ExecBatchRequest,
    ExecRequest,
    ExecResponse,
    ExplainRequest,
    HealthRequest,
    HealthResponse,
)
from .gen.qql_connect import QQLClientSync


@dataclass
class Result:
    """Outcome of a single QQL query execution."""

    ok: bool
    operation: str
    message: str
    data: Optional[Any] = None

    @classmethod
    def from_proto(cls, resp: ExecResponse) -> "Result":
        data = None
        if resp.data:
            try:
                data = json.loads(resp.data.decode("utf-8"))
            except (json.JSONDecodeError, UnicodeDecodeError):
                data = resp.data
        return cls(
            ok=resp.ok,
            operation=resp.operation,
            message=resp.message,
            data=data,
        )

    @classmethod
    def error(cls, err: Exception) -> "Result":
        return cls(ok=False, operation="", message=str(err))


@dataclass
class HealthStatus:
    """Gateway and Qdrant connection status."""

    version: str
    qdrant_connected: bool
    qdrant_status: str

    @classmethod
    def from_proto(cls, resp: HealthResponse) -> "HealthStatus":
        return cls(
            version=resp.version,
            qdrant_connected=resp.qdrant_connected,
            qdrant_status=resp.qdrant_status,
        )


class QQLClient:
    """Client for the QQL Gateway (Connect RPC).

    Uses the generated Connect RPC client under the hood.
    Exposes simple string-in/Result-out methods for common usage,
    and a `.raw` property for direct protobuf access.

    Args:
        url: Gateway endpoint (default ``http://localhost:50051``).
        api_key: Optional Bearer token for authentication.
    """

    def __init__(self, url: str = "http://localhost:50051", api_key: Optional[str] = None):
        self.url = url
        self._api_key = api_key
        self._client = QQLClientSync(url)

    @property
    def raw(self) -> QQLClientSync:
        """Access the raw Connect RPC client for direct protobuf usage."""
        return self._client

    def exec(self, query: str) -> Result:
        """Execute a single QQL query.

        Args:
            query: QQL statement, e.g. ``"QUERY 'search' FROM docs LIMIT 5"``.

        Returns:
            Result with parsed JSON data on success.
        """
        try:
            resp = self._client.exec(ExecRequest(query=query))
            return Result.from_proto(resp)
        except Exception as exc:
            return Result.error(exc)

    def exec_batch(self, queries: List[str], stop_on_error: bool = False) -> List[Result]:
        """Execute multiple QQL queries in one round-trip.

        Args:
            queries: List of QQL statements.
            stop_on_error: If True, stop at the first error.

        Returns:
            List of Results (one per query).
        """
        try:
            req = ExecBatchRequest(
                queries=[ExecRequest(query=q) for q in queries],
                stop_on_error=stop_on_error,
            )
            resp = self._client.exec_batch(req)
            return [Result.from_proto(r) for r in resp.results]
        except Exception as exc:
            return [Result.error(exc)]

    def explain(self, query: str) -> str:
        """Return the execution plan for a query without running it.

        Args:
            query: QQL statement to explain.

        Returns:
            Human-readable execution plan string.
        """
        resp = self._client.explain(ExplainRequest(query=query))
        return resp.plan

    def health(self) -> HealthStatus:
        """Check gateway and Qdrant connection status.

        Returns:
            HealthStatus with version and connection info.
        """
        resp = self._client.health(HealthRequest())
        return HealthStatus.from_proto(resp)

    def close(self) -> None:
        """Close the underlying connection."""
        self._client.close()

    def __enter__(self) -> "QQLClient":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()
