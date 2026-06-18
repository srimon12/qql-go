"""Policy Editor tab — view and edit policies.yaml."""

import streamlit as st
import yaml

from utils import POLICY_FILE


def render():
    st.header("📝 Policy Editor")
    st.markdown("View and edit the gateway policy. Changes take effect on next request if `--policy-reload` is enabled.")

    col1, col2 = st.columns([2, 1])

    with col1:
        policy_content = ""
        if POLICY_FILE.exists():
            policy_content = POLICY_FILE.read_text()

        edited_policy = st.text_area(
            "policies.yaml",
            value=policy_content,
            height=600,
            key="policy_editor",
        )

    with col2:
        st.subheader("Parsed Rules")
        try:
            parsed = yaml.safe_load(edited_policy)
            rules = parsed.get("rules", [])
            for i, rule in enumerate(rules):
                _render_rule(i, rule)
        except yaml.YAMLError as e:
            st.error(f"YAML parse error: {e}")

        if st.button("💾 Save Policy", type="primary"):
            try:
                yaml.safe_load(edited_policy)
                POLICY_FILE.write_text(edited_policy)
                st.success("Policy saved. Gateway will reload if --policy-reload is enabled.")
            except yaml.YAMLError as e:
                st.error(f"Cannot save — YAML error: {e}")


def _render_rule(i: int, rule: dict):
    match = rule.get("match", {})
    allow = rule.get("allow", [])
    collections = rule.get("collections", [])
    inject = rule.get("inject", {})
    limits = rule.get("limits", {})

    with st.expander(f"Rule {i+1}", expanded=False):
        if match.get("claims"):
            for k, v in match["claims"].items():
                st.caption(f"match.{k} = `{v}`")
        if match.get("authenticated"):
            st.caption("match: any authenticated user")
        if allow:
            st.caption(f"allow: {', '.join(allow)}")
        if collections:
            st.caption(f"collections: {', '.join(collections)}")
        if inject.get("where"):
            w = inject["where"]
            st.caption(f"inject: {w.get('field', '?')} {w.get('op', '=')} {w.get('from_claim', w.get('value', '?'))}")
        if limits.get("max_limit"):
            st.caption(f"max_limit: {limits['max_limit']}")
