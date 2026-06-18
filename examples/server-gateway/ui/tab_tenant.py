"""Tenant Isolation tab — same query, different users, side-by-side comparison."""

import streamlit as st

from utils import GW_URL, DEMO_USERS, api_post, login, scroll_points

# All demo users with their expected access
USER_PROFILES = {
    "alice@acme.com": {
        "password": "alice123",
        "name": "Alice Chen",
        "role": "reader",
        "org": "acme-corp",
        "dept": "engineering",
        "label": "Alice — reader, acme-corp, engineering",
    },
    "bob@acme.com": {
        "password": "bob123",
        "name": "Bob Smith",
        "role": "admin",
        "org": "acme-corp",
        "dept": "engineering",
        "label": "Bob — admin, acme-corp",
    },
    "carol@globex.com": {
        "password": "carol123",
        "name": "Carol Rivera",
        "role": "reader",
        "org": "globex-corp",
        "dept": "finance",
        "label": "Carol — reader, globex-corp, finance",
    },
    "dave@acme.com": {
        "password": "dave123",
        "name": "Dave Patel",
        "role": "reader",
        "org": "acme-corp",
        "dept": "finance",
        "label": "Dave — reader, acme-corp, finance",
    },
    "eve@globex.com": {
        "password": "eve123",
        "name": "Eve Nakamura",
        "role": "manager",
        "org": "globex-corp",
        "dept": "engineering",
        "label": "Eve — manager, globex-corp, engineering",
    },
}


def render():
    st.header("🏢 Tenant Isolation Demo")
    st.markdown("""
    **Same query. Different users. Different results.**

    The gateway rewrites every query at the AST level before it reaches Qdrant.
    Forbidden documents are never scored, never retrieved, never in the prompt.

    **Policy rules applied:**
    - **Readers**: `WHERE tenant_id = <org> AND department IN (<dept>, all) AND access_level != 'confidential'`
    - **Admins**: `WHERE tenant_id = <org>` (sees all departments, all access levels)
    - **Managers**: `WHERE tenant_id = <org>` (read + write)
    - **Agents**: `WHERE access_level = 'public'` (public docs only, any tenant)
    """)

    # Query input
    query = st.text_input(
        "Query to compare",
        value="SCROLL FROM docs LIMIT 20",
        help="Try: SCROLL FROM docs LIMIT 20, or QUERY 'company' FROM docs LIMIT 5 USING HYBRID",
    )

    # User selection
    st.subheader("Select users to compare")
    selected_emails = st.multiselect(
        "Pick 2-5 users",
        options=list(USER_PROFILES.keys()),
        default=["alice@acme.com", "dave@acme.com", "bob@acme.com"],
        format_func=lambda x: USER_PROFILES[x]["label"],
        max_selections=5,
    )

    if len(selected_emails) < 2:
        st.warning("Select at least 2 users to compare")
        return

    if st.button("▶ Run comparison", type="primary", use_container_width=True):
        _run_comparison(query, selected_emails)


def _run_comparison(query: str, emails: list[str]):
    """Run the same query as each selected user and show side-by-side results."""
    n = len(emails)
    cols = st.columns(n)

    for i, email in enumerate(emails):
        profile = USER_PROFILES[email]
        with cols[i]:
            st.subheader(profile["name"])
            st.caption(f"`{profile['org']}` · {profile['role']} · {profile['dept']}")

            # Login
            login_result = login(email, profile["password"])
            if not login_result or "token" not in login_result:
                st.error("Login failed")
                continue

            token = login_result["token"]
            user_info = login_result.get("user", {})

            # Execute query
            with st.spinner(f"Querying as {profile['name']}..."):
                result = api_post(f"{GW_URL}/qql.QQL/Exec", {"query": query}, token)

            if result.get("ok"):
                pts = scroll_points(result)
                st.metric("Documents found", len(pts))

                if pts:
                    for pt in pts:
                        p = pt.get("payload", {})
                        src = p.get("source", "?")
                        dept = p.get("department", "?")
                        acc = p.get("access_level", "?")
                        tid = p.get("tenant_id", "?")
                        text = p.get("text", "")

                        with st.expander(f"{src} — {tid}/{dept}/{acc}"):
                            st.write(text)
                            st.caption(f"tenant: {tid} · dept: {dept} · access: {acc}")
                else:
                    st.info("No documents returned")
            else:
                error_msg = result.get("message", result.get("error", "failed"))
                if "not permitted" in error_msg.lower():
                    st.error(f"🚫 Policy denied: {error_msg}")
                elif "authentication" in error_msg.lower():
                    st.error(f"🔒 {error_msg}")
                else:
                    st.error(f"❌ {error_msg}")
