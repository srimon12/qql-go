import os
import sys

# Add the 'gen' directory to sys.path so generated protobufs can import each other
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen"))

from .client import QQLClient, Result, HealthStatus

__all__ = ["QQLClient", "Result", "HealthStatus"]
