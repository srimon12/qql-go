"""
qql-go Gateway Dashboard

Streamlit UI for demonstrating and interacting with the qql-go gateway.

Usage:
    uv run streamlit run ui/app.py
"""

import streamlit as st

from utils import AUTH_URL, GW_URL, DEMO_USERS, login

# Page config
st.set_page_config(
    page_title="qql-go Gateway",
    page_icon="🔐",
    layout="wide",
    initial_sidebar_state="expanded",
)

# Session state
if "token" not in st.session_state:
    st.session_state.token = None
if "user" not in st.session_state:
    st.session_state.user = None

# ── Sidebar: Login ──────────────────────────────────────────────────
with st.sidebar:
    st.title("🔐 qql-go Gateway")
    st.caption("Policy-enforced retrieval for Qdrant")
    st.divider()

    if st.session_state.user:
        u = st.session_state.user
        st.success(f"Logged in as **{u.get('name', u.get('email', '?'))}**")
        st.caption(f"`{u.get('email', u.get('sub', '?'))}`")
        col1, col2 = st.columns(2)
        col1.metric("Role", u.get("role", "-"))
        col2.metric("Org", u.get("org_id", "-"))
        if u.get("department"):
            st.metric("Department", u.get("department", "-"))
        if st.button("Logout", use_container_width=True):
            st.session_state.token = None
            st.session_state.user = None
            st.rerun()
    else:
        st.subheader("Login")
        login_user = st.selectbox(
            "Select user",
            options=list(DEMO_USERS.keys()),
            format_func=lambda x: DEMO_USERS[x]["label"],
        )
        if st.button("Login", use_container_width=True, type="primary"):
            result = login(login_user, DEMO_USERS[login_user]["password"])
            if result and "token" in result:
                st.session_state.token = result["token"]
                st.session_state.user = result["user"]
                st.rerun()
            else:
                st.error("Login failed")

    st.divider()
    st.caption("Services")
    st.code(f"Auth: {AUTH_URL}\nGateway: {GW_URL}")

# ── Tabs ────────────────────────────────────────────────────────────
import tab_query
import tab_explain
import tab_tenant
import tab_templates
import tab_policy
import tab_audit
import tab_convert
import tab_sec_playground

tab_query_tab, tab_explain_tab, tab_tenant_tab, tab_templates_tab, tab_policy_tab, tab_audit_tab, tab_convert_tab, tab_sec_playground_tab = st.tabs([
    "🔍 Query",
    "📋 Explain",
    "🏢 Tenant Isolation",
    "📦 Templates",
    "📝 Policy Editor",
    "📊 Audit Log",
    "🔄 Convert",
    "⚡ Security Playground",
])

with tab_query_tab:
    tab_query.render()

with tab_explain_tab:
    tab_explain.render()

with tab_tenant_tab:
    tab_tenant.render()

with tab_templates_tab:
    tab_templates.render()

with tab_policy_tab:
    tab_policy.render()

with tab_audit_tab:
    tab_audit.render()

with tab_convert_tab:
    tab_convert.render()

with tab_sec_playground_tab:
    tab_sec_playground.render()
