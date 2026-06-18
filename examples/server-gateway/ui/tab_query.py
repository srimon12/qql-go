"""Query tab — execute QQL with policy enforcement."""

import streamlit as st

from utils import GW_URL, api_post, decode_data, scroll_points


def render():
    st.header("Execute QQL")
    st.markdown("Send a QQL query through the gateway. The gateway validates your JWT, enforces policy, injects tenant filters, and executes against Qdrant.")

    col1, col2 = st.columns([3, 1])
    with col1:
        default_query = st.session_state.pop("pending_query", "QUERY 'company' FROM docs LIMIT 5 USING HYBRID")
        query = st.text_area(
            "QQL Query",
            value=default_query,
            height=100,
            key="query_input",
        )
    with col2:
        st.markdown("###")
        run_query = st.button("▶ Execute", type="primary", use_container_width=True)

    if run_query and query:
        if not st.session_state.token:
            st.error("Please login first")
        else:
            with st.spinner("Executing..."):
                result = api_post(f"{GW_URL}/qql.QQL/Exec", {"query": query}, st.session_state.token)

            if result.get("ok"):
                st.success(result.get("message", "OK"))
                _show_result(result)
            else:
                error_msg = result.get("message", result.get("error", "Unknown error"))
                if "not permitted" in error_msg.lower():
                    st.error(f"🚫 Policy denied: {error_msg}")
                elif "authentication" in error_msg.lower():
                    st.error(f"🔒 Auth failed: {error_msg}")
                else:
                    st.error(f"❌ {error_msg}")

    st.divider()
    st.subheader("Quick Queries")
    col1, col2, col3 = st.columns(3)
    with col1:
        if st.button("SHOW COLLECTIONS"):
            st.session_state["pending_query"] = "SHOW COLLECTIONS"
            st.rerun()
    with col2:
        if st.button("SCROLL docs LIMIT 10"):
            st.session_state["pending_query"] = "SCROLL FROM docs LIMIT 10"
            st.rerun()
    with col3:
        if st.button("QUERY 'search' USING HYBRID"):
            st.session_state["pending_query"] = "QUERY 'search' FROM docs LIMIT 5 USING HYBRID"
            st.rerun()


def _show_result(result: dict):
    data = decode_data(result.get("data"))
    if not data:
        return

    # QUERY results: list of {id, score, text, payload: {...}}
    if isinstance(data, list) and len(data) > 0 and isinstance(data[0], dict):
        rows = []
        for item in data:
            row = {}
            # Start with payload fields (the useful stuff)
            if item.get("payload"):
                row.update(item["payload"])
            # Add id, score, text at the end
            row["id"] = item.get("id")
            if item.get("score"):
                row["score"] = item.get("score")
            if item.get("text") and "text" not in row:
                row["text"] = item["text"]
            rows.append(row)
        st.dataframe(rows, use_container_width=True)
        return

    # SCROLL results: dict with points: [{id, payload: {...}}]
    if isinstance(data, dict) and "points" in data:
        rows = []
        for item in data["points"]:
            row = {}
            if item.get("payload"):
                row.update(item["payload"])
            row["id"] = item.get("id")
            rows.append(row)
        st.dataframe(rows, use_container_width=True)
        return

    # Fallback
    st.json(data)
