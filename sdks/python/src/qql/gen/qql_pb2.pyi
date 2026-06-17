from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ConvertRequest(_message.Message):
    __slots__ = ("json_payload",)
    JSON_PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    json_payload: str
    def __init__(self, json_payload: _Optional[str] = ...) -> None: ...

class ConvertResponse(_message.Message):
    __slots__ = ("ok", "statements", "error")
    OK_FIELD_NUMBER: _ClassVar[int]
    STATEMENTS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    ok: bool
    statements: _containers.RepeatedScalarFieldContainer[str]
    error: str
    def __init__(self, ok: _Optional[bool] = ..., statements: _Optional[_Iterable[str]] = ..., error: _Optional[str] = ...) -> None: ...

class ExecRequest(_message.Message):
    __slots__ = ("query",)
    QUERY_FIELD_NUMBER: _ClassVar[int]
    query: str
    def __init__(self, query: _Optional[str] = ...) -> None: ...

class ExecResponse(_message.Message):
    __slots__ = ("ok", "operation", "message", "data")
    OK_FIELD_NUMBER: _ClassVar[int]
    OPERATION_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    ok: bool
    operation: str
    message: str
    data: bytes
    def __init__(self, ok: _Optional[bool] = ..., operation: _Optional[str] = ..., message: _Optional[str] = ..., data: _Optional[bytes] = ...) -> None: ...

class ExecBatchRequest(_message.Message):
    __slots__ = ("queries", "stop_on_error")
    QUERIES_FIELD_NUMBER: _ClassVar[int]
    STOP_ON_ERROR_FIELD_NUMBER: _ClassVar[int]
    queries: _containers.RepeatedCompositeFieldContainer[ExecRequest]
    stop_on_error: bool
    def __init__(self, queries: _Optional[_Iterable[_Union[ExecRequest, _Mapping]]] = ..., stop_on_error: _Optional[bool] = ...) -> None: ...

class ExecBatchResponse(_message.Message):
    __slots__ = ("results",)
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    results: _containers.RepeatedCompositeFieldContainer[ExecResponse]
    def __init__(self, results: _Optional[_Iterable[_Union[ExecResponse, _Mapping]]] = ...) -> None: ...

class ExplainRequest(_message.Message):
    __slots__ = ("query", "json")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    JSON_FIELD_NUMBER: _ClassVar[int]
    query: str
    json: bool
    def __init__(self, query: _Optional[str] = ..., json: _Optional[bool] = ...) -> None: ...

class ExplainResponse(_message.Message):
    __slots__ = ("ok", "query", "plan")
    OK_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    PLAN_FIELD_NUMBER: _ClassVar[int]
    ok: bool
    query: str
    plan: str
    def __init__(self, ok: _Optional[bool] = ..., query: _Optional[str] = ..., plan: _Optional[str] = ...) -> None: ...

class HealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthResponse(_message.Message):
    __slots__ = ("version", "qdrant_connected", "qdrant_status")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    QDRANT_CONNECTED_FIELD_NUMBER: _ClassVar[int]
    QDRANT_STATUS_FIELD_NUMBER: _ClassVar[int]
    version: str
    qdrant_connected: bool
    qdrant_status: str
    def __init__(self, version: _Optional[str] = ..., qdrant_connected: _Optional[bool] = ..., qdrant_status: _Optional[str] = ...) -> None: ...
