"""Explain tab — view query plans without execution."""

import streamlit as st

from utils import GW_URL, api_post


def render():
    st.header("Query Plan")
    st.markdown("See how the gateway parses and plans your query without executing it.")

    explain_query = st.text_area(
        "QQL Query",
        value="QUERY 'emergency care' FROM docs LIMIT 10 USING HYBRID RERANK",
        height=80,
        key="explain_input",
    )

    if st.button("📋 Explain", type="primary"):
        if not st.session_state.token:
            st.error("Please login first")
        else:
            result = api_post(f"{GW_URL}/qql.QQL/Explain", {"query": explain_query}, st.session_state.token)
            if result.get("ok"):
                st.code(result.get("plan", ""), language="text")
            else:
                st.error(result.get("message", result.get("error", "Explain failed")))
