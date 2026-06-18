"""Audit Log tab — structured audit entries with filtering."""

import json
import os

import streamlit as st


def render():
    st.header("📊 Audit Trail")
    st.markdown("Structured audit entries from the gateway. Shows who queried what, what was injected, and what was denied.")

    audit_file = os.getenv("QQL_AUDIT_FILE", "")
    audit_entries = _load_entries(audit_file)

    if not audit_entries:
        st.info("No audit entries yet. Run some queries through the gateway.")
        if audit_file:
            st.caption(f"Reading from: {audit_file}")
        else:
            st.caption("Set QQL_AUDIT_FILE environment variable to the audit log path.")
        return

    # Metrics
    allowed = sum(1 for e in audit_entries if e.get("allowed"))
    denied = sum(1 for e in audit_entries if e.get("denied"))
    errors = sum(1 for e in audit_entries if e.get("status") == "error")

    st.metric("Total Requests", len(audit_entries))
    col1, col2, col3 = st.columns(3)
    col1.metric("✅ Allowed", allowed)
    col2.metric("🚫 Denied", denied)
    col3.metric("❌ Errors", errors)

    st.divider()

    # Filter
    status_filter = st.selectbox("Filter by status", ["All", "ok", "denied", "error"])
    filtered = audit_entries
    if status_filter != "All":
        filtered = [e for e in audit_entries if e.get("status") == status_filter]

    # Entries
    for entry in reversed(filtered):
        status = entry.get("status", "?")
        icon = "✅" if status == "ok" else "🚫" if status == "denied" else "❌"
        ts = entry.get("ts", "")[:19]
        subject = entry.get("subject", "-") or "-"
        op = entry.get("operation", "?")
        collection = entry.get("collection", "-") or "-"
        latency = entry.get("latency_ms", "")
        reason = entry.get("denied_reason", "") or ""

        with st.expander(f"{icon} {op} — {subject} — {collection} — {status}"):
            col1, col2, col3 = st.columns(3)
            col1.caption(f"Time: {ts}")
            col2.caption(f"Latency: {latency}ms")
            col3.caption(f"Rule: {entry.get('rule_index', '-')}")
            if reason:
                st.error(f"Denied: {reason}")
            if entry.get("query"):
                st.code(entry["query"], language="sql")
            if entry.get("filters_injected"):
                st.info(f"Injected: {', '.join(entry['filters_injected'])}")
            st.json(entry)


def _load_entries(audit_file: str) -> list:
    entries = []
    if not audit_file or not os.path.exists(audit_file):
        return entries
    with open(audit_file) as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    entries.append(json.loads(line))
                except json.JSONDecodeError:
                    pass
    return entries
