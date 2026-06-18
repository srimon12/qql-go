"""Templates tab — named operations with variable substitution."""

import re

import streamlit as st
import yaml

from utils import GW_URL, TEMPLATE_FILE, api_post, decode_data


def render():
    st.header("📦 Query Templates")
    st.markdown("Invoke named operations instead of writing raw QQL. Variables are substituted from params and JWT claims.")

    templates = _load_templates()
    if not templates:
        st.warning(f"No templates found at {TEMPLATE_FILE}")
        return

    selected = st.selectbox("Select template", options=list(templates.keys()))
    tmpl = templates[selected]

    st.info(f"**{tmpl.get('description', '')}**")
    st.code(tmpl.get("query", ""), language="sql")

    # Extract {variables} (excluding claims.*)
    params = [p for p in re.findall(r"\{(\w+)\}", tmpl.get("query", "")) if not p.startswith("claims.")]

    param_values = {}
    if params:
        st.subheader("Parameters")
        for p in params:
            param_values[p] = st.text_input(f"{{{p}}}", key=f"tmpl_{p}")

    if st.button("▶ Execute Template", type="primary"):
        if not st.session_state.token:
            st.error("Please login first")
            return

        query_str = tmpl.get("query", "")
        for k, v in param_values.items():
            query_str = query_str.replace(f"{{{k}}}", v)
        if st.session_state.user:
            for k, v in st.session_state.user.items():
                query_str = query_str.replace(f"{{claims.{k}}}", str(v))

        st.code(f"Resolved: {query_str}", language="sql")

        result = api_post(f"{GW_URL}/qql.QQL/Exec", {"query": query_str}, st.session_state.token)
        if result.get("ok"):
            st.success(result.get("message", "OK"))
            data = decode_data(result.get("data"))
            if data:
                if isinstance(data, list):
                    st.dataframe(data, use_container_width=True)
                elif isinstance(data, dict) and "points" in data:
                    st.dataframe(data["points"], use_container_width=True)
                else:
                    st.json(data)
        else:
            st.error(result.get("message", result.get("error", "failed")))


def _load_templates() -> dict:
    if TEMPLATE_FILE.exists():
        with open(TEMPLATE_FILE) as f:
            cfg = yaml.safe_load(f) or {}
            return cfg.get("templates", {})
    return {}
