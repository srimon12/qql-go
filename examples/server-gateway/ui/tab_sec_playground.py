"""Gateway Security & Policy Playground tab."""

import json
from pathlib import Path
import streamlit as st

from utils import GW_URL, api_post, decode_data
from tab_convert import parse_qdrant_payload, run_qdrant_rest, extract_points_from_rest, extract_points_from_qql, scores_match

SECURITY_EXAMPLES_FILE = Path(__file__).parent.parent / "security_examples.json"


def load_security_examples() -> dict:
    """Load security examples from JSON file."""
    if SECURITY_EXAMPLES_FILE.exists():
        with open(SECURITY_EXAMPLES_FILE) as f:
            return json.load(f)
    return {}


def render():
    st.header("🔐 Gateway Security & Policy Playground")
    st.markdown("""
    This playground demonstrates the **Gateway security, policy injection, and query mutation layer** over our seeded `docs` collection. 
    Choose a complex query example, edit the JSON payload, and run both direct and gateway queries to see the effects of security policies.
    """)

    # Load examples from file
    security_examples = load_security_examples()

    # --- Use Global Login Token ---
    token = st.session_state.get("token")
    claims = st.session_state.get("user") or {}

    if not token:
        st.warning("⚠️ Please login from the sidebar to use the Security Playground.")
        return

    st.info(f"**Logged in as:** {claims.get('name', claims.get('user_id', '-'))} | Tenant: `{claims.get('org_id', '-')}` | Department: `{claims.get('department', '-')}` | Role: `{claims.get('role', '-')}`")

    st.divider()

    # --- Input Mode Selector ---
    st.subheader("1. Choose Input Mode")
    input_mode = st.radio(
        "Select input type",
        ["JSON Payload", "QQL Statement"],
        horizontal=True,
        key="sec_input_mode"
    )

    if input_mode == "JSON Payload":
        # --- Example Selector ---
        st.subheader("2. Select or Edit JSON Payload")
        example_key = st.selectbox(
            "Choose a complex query payload",
            options=list(security_examples.keys()),
            key="sec_example_select"
        )
        
        example_details = security_examples[example_key]
        st.info(f"💡 **Description:** {example_details['description']}")

        # Keep track of JSON state in Streamlit session
        json_key = f"sec_json_editor_{example_key}"
        if json_key not in st.session_state:
            st.session_state[json_key] = json.dumps(example_details["json"], indent=2)

        col_editor, col_qql = st.columns([1, 1])

        with col_editor:
            st.markdown("**📝 Qdrant REST JSON Payload**")
            source_json = st.text_area(
                "JSON payload editor",
                height=300,
                key=json_key,
                label_visibility="collapsed"
            )
            
        with col_qql:
            st.markdown("**💻 Converted QQL Statement**")
            # Pre-convert JSON to QQL (Auth-less Convert RPC — build a wrapped
            # payload so the converter extracts the collection from the path)
            try:
                method, path, body = parse_qdrant_payload(source_json, "docs")
                wrapped = json.dumps({
                    "method": method,
                    "path": "/" + path.lstrip("/"),
                    "body": body,
                })
                conv_resp = api_post(
                    f"{GW_URL}/qql.QQL/Convert",
                    {"jsonPayload": wrapped},
                    token
                )
                if conv_resp.get("ok"):
                    statements = conv_resp.get("statements", [])
                    qql_stmt = statements[0] if statements else ""
                    st.code(qql_stmt, language="sql")
                else:
                    qql_stmt = ""
                    st.error(f"Translation failed: {conv_resp.get('error', conv_resp.get('message'))}")
            except Exception as e:
                qql_stmt = ""
                st.error(f"Translation failed: {e}")

    else:  # QQL Statement mode
        st.subheader("2. Enter QQL Statement")
        st.markdown("Enter a QQL statement directly to see how the Gateway applies policy injection.")
        
        qql_input = st.text_area(
            "QQL Statement",
            value="QUERY 'microservices architecture' FROM docs USING 'dense' WITH MODEL 'text-embedding-all-minilm-l6-v2-embedding' LIMIT 5",
            height=150,
            key="sec_qql_input",
            help="Enter a QQL query to see how the Gateway transforms it with policy filters."
        )
        
        qql_stmt = qql_input.strip()
        source_json = ""  # No JSON source in QQL mode

    # --- Run Query ---
    st.markdown("### 3. Run Query & Compare Results")
    run_btn = st.button("⚡ Execute Query", type="primary", use_container_width=True)

    if run_btn and qql_stmt:
        with st.spinner("Executing queries..."):
            # Get Mutated Compiled Request from Gateway (Explain with JWT token)
            explain_resp = api_post(f"{GW_URL}/qql.QQL/Explain", {"query": qql_stmt, "json": True}, token)
            mutated_json = explain_resp.get("plan", "{}") if explain_resp.get("ok") else json.dumps({"error": explain_resp.get("message")})
            
            # Execute QQL query on Gateway (Routes via Gateway with JWT token)
            gateway_resp = api_post(f"{GW_URL}/qql.QQL/Exec", {"query": qql_stmt}, token)

            # Run direct Qdrant REST only in JSON mode
            if source_json:
                method, path, body = parse_qdrant_payload(source_json, "docs")
                rest_resp = run_qdrant_rest(method, path, body)
            else:
                rest_resp = None
                body = {}

        # --- Side-by-side Payload Mutation Analysis ---
        st.subheader("📋 Policy Transformation Analysis")
        
        if input_mode == "QQL Statement":
            st.caption("See how the Gateway transforms your QQL by injecting security policies based on your JWT claims.")
            
            col_orig, col_gate = st.columns(2)
            with col_orig:
                st.markdown("**Original QQL Statement**")
                st.caption("Your input query")
                st.code(qql_stmt, language="sql")
            with col_gate:
                st.markdown("**Gateway Mutated Request**")
                st.caption("After policy injection (from Explain)")
                try:
                    st.json(json.loads(mutated_json))
                except Exception:
                    st.code(mutated_json, language="json")
        else:
            st.caption("Compare the original REST JSON versus the mutated gRPC payload after JWT claim boundaries are applied.")
            
            col_mut_orig, col_mut_gate = st.columns(2)
            with col_mut_orig:
                st.markdown("**Original REST JSON**")
                st.caption("Plain payload without filters")
                st.json(body)
            with col_mut_gate:
                st.markdown("**Gateway Mutated REST JSON**")
                st.caption("Compiled by compiler (includes resolved names & policy filters)")
                try:
                    st.json(json.loads(mutated_json))
                except Exception:
                    st.code(mutated_json, language="json")

        st.divider()

        # --- Live Execution Output ---
        st.subheader("⚡ Live Execution")
        
        qql_pts = extract_points_from_qql(decode_data(gateway_resp.get("data")))
        qql_ids = [str(p.get("id")) for p in qql_pts if p.get("id") is not None]
        is_gateway_err = not gateway_resp.get("ok")

        if is_gateway_err:
            st.error(f"Gateway execution error: {gateway_resp.get('message')}")
        else:
            st.success(f"✅ Gateway returned {len(qql_ids)} points")

        if rest_resp is not None:
            # JSON mode: show side-by-side comparison
            st.caption("Compare direct Qdrant vs Gateway with policy enforcement.")
            
            rest_pts = extract_points_from_rest(rest_resp)
            rest_ids = [str(p.get("id")) for p in rest_pts if p.get("id") is not None]
            
            is_rest_err = isinstance(rest_resp, str) or (isinstance(rest_resp, dict) and ("error" in rest_resp or (isinstance(rest_resp.get("status"), dict) and rest_resp["status"].get("error"))))

            if not is_rest_err and not is_gateway_err:
                if rest_ids and qql_ids:
                    if set(rest_ids) == set(qql_ids):
                        st.info("Same documents returned (policies didn't filter)")
                    else:
                        st.warning(f"Policy filtered results: Direct={len(rest_ids)}, Gateway={len(qql_ids)}")
                        st.info(f"""
                        💡 **Policy Enforcement:** You queried as **{claims.get('name', claims.get('user_id', '-'))}** (Tenant: `{claims.get('org_id')}`, Dept: `{claims.get('department')}`).
                        - Direct: **{len(rest_ids)}** points {rest_ids}
                        - Gateway: **{len(qql_ids)}** points {qql_ids}
                        """)

            col_res_rest, col_res_gate = st.columns(2)
            with col_res_rest:
                st.markdown("**Direct Qdrant REST**")
                st.caption("No policy filters")
                if rest_ids:
                    st.metric("Points", len(rest_ids))
                st.json(rest_resp)

            with col_res_gate:
                st.markdown("**QQL Gateway**")
                st.caption("With JWT policies")
                if qql_ids:
                    st.metric("Points", len(qql_ids))
                st.json({
                    "ok": gateway_resp.get("ok"),
                    "operation": gateway_resp.get("operation"),
                    "message": gateway_resp.get("message"),
                    "data": decode_data(gateway_resp.get("data"))
                })
        else:
            # QQL mode: show only gateway output
            st.caption("Gateway execution output with policy filters applied.")
            
            if qql_ids:
                st.metric("Points Returned", len(qql_ids))
                st.write("IDs:", qql_ids)
            st.json({
                "ok": gateway_resp.get("ok"),
                "operation": gateway_resp.get("operation"),
                "message": gateway_resp.get("message"),
                "data": decode_data(gateway_resp.get("data"))
            })
