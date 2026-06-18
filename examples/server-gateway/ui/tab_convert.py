"""Convert tab — Qdrant REST JSON → QQL with 110 categorized examples."""

import json

import streamlit as st

from utils import GW_URL, api_post, load_payload_examples

# Category definitions — covers all 110 payloads in all_payloads.json
CATEGORIES = {
    "Query — Score Boosting & Formula": ["score boost", "geo distance", "time-based", "formula"],
    "Query — MMR, Relevance Feedback, Sample": ["mmr", "relevance", "random sample"],
    "Query — Search (vector, text, named, sparse)": ["search with", "query with", "sparse", "search by id", "acorn", "lookup_from"],
    "Query — Batch & Pagination": ["batch", "pagination", "offset"],
    "Query — Grouped Retrieval": ["group"],
    "Query — Recommend & Discover & Context": ["recommend", "discover", "context"],
    "Hybrid & Fusion (RRF, DBSF)": ["rrf", "dbsf", "fusion", "weighted"],
    "Multi-Stage Prefetch (1/2/3 level)": ["prefetch", "multi-stage", "rescore"],
    "Multi-Vector & PDF Retrieval": ["multivector", "multi-vector", "colbert", "pdf", "2d query"],
    "Filters (must, should, range, geo, nested)": ["filter"],
    "Collection & Index Creation": ["create", "index"],
    "Insert & Upsert": ["insert", "upsert"],
    "Delete, Scroll, Get, Set Payload": ["delete", "scroll", "get point", "set payload", "distance matrix"],
    "Wrapped Endpoints": ["wrapped"],
    "Complex Payloads (E-Commerce, Healthcare, IoT)": ["e-commerce", "healthcare", "financial", "smart city", "media", "complex"],
}

# Exclusion rules for overlapping categories
_EXCLUDE = {
    "Query — Search (vector, text, named, sparse)": ["batch", "group", "prefetch"],
    "Query — Recommend & Discover & Context": ["batch"],
    "Insert & Upsert": ["delete"],
}


def render():
    st.header("🔄 REST JSON → QQL Converter")
    st.markdown("""
    **Paste Qdrant REST API JSON** — the same JSON you'd send to Qdrant's HTTP endpoints — and get equivalent QQL statements.

    This is a migration tool. If you have existing code using the Qdrant Python SDK, REST API, or any HTTP client,
    Convert shows you the equivalent QQL. It works **offline** — no Qdrant connection needed for the conversion itself.

    **Supported:** `PUT /collections` → `CREATE COLLECTION`, `PUT /points` → `INSERT`, `POST /query` → `QUERY`,
    `POST /scroll` → `SCROLL`, `POST /recommend` → `QUERY RECOMMEND`, `DELETE /points` → `DELETE`
    """)

    payload_examples = load_payload_examples()

    # ── Left: Input ──────────────────────────────────────────────────
    st.subheader("Input: Qdrant REST JSON")

    # Example picker
    col_cat, col_ex = st.columns(2)
    with col_cat:
        category = st.selectbox("Category", options=list(CATEGORIES.keys()), key="convert_category")
    with col_ex:
        patterns = CATEGORIES[category]
        excludes = _EXCLUDE.get(category, [])
        example_names = [
            k for k in payload_examples
            if any(p in k.lower() for p in patterns)
            and not any(e in k.lower() for e in excludes)
        ]
        selected = st.selectbox(
            "Load example",
            options=["— paste custom JSON —"] + example_names,
            key="convert_example",
        )

    # Show selected example info
    if selected != "— paste custom JSON —":
        st.caption(f"📌 {selected}")

    # Text area — user can paste custom JSON or edit
    json_input = st.text_area(
        "JSON payload",
        value=st.session_state.get("convert_json_content", _default_json()),
        height=250,
        key="convert_json_input",
        help="Paste Qdrant REST API JSON, or select an example above and click Convert.",
    )

    convert_clicked = st.button("🔄 Convert to QQL", type="primary", use_container_width=True)

    # ── Right: Output ────────────────────────────────────────────────
    if convert_clicked:
        _do_convert(json_input, payload_examples, selected)
    elif selected != "— paste custom JSON —":
        # Show the example JSON nicely formatted on the right
        st.subheader(f"Example: {selected}")
        example_json = payload_examples[selected]
        try:
            st.json(json.loads(example_json))
        except json.JSONDecodeError:
            st.code(example_json, language="json")
        st.caption("Click **Convert to QQL** to see the equivalent QQL statements.")


def _do_convert(json_input: str, payload_examples: dict, selected: str):
    """Run the conversion and display results."""
    # If user picked an example and didn't edit the text area, use the example directly
    if selected != "— paste custom JSON —" and selected in payload_examples:
        source_json = payload_examples[selected]
    else:
        source_json = json_input

    if not source_json.strip():
        st.warning("Paste some JSON or select an example first")
        return

    with st.spinner("Converting..."):
        result = api_post(f"{GW_URL}/qql.QQL/Convert", {"jsonPayload": source_json}, st.session_state.get("token"))

    if not result.get("ok"):
        st.error(result.get("error", result.get("message", "Convert failed")))
        return

    statements = result.get("statements", [])
    st.success(f"Generated {len(statements)} QQL statement(s)")

    for i, stmt in enumerate(statements):
        st.code(stmt, language="sql")

    # Execute option
    if st.session_state.get("token"):
        st.divider()
        st.subheader("Execute converted QQL")
        for i, stmt in enumerate(statements):
            col_a, col_b = st.columns([5, 1])
            with col_a:
                st.code(stmt, language="sql")
            with col_b:
                if st.button("▶ Run", key=f"exec_conv_{i}"):
                    with st.spinner("Executing..."):
                        exec_result = api_post(f"{GW_URL}/qql.QQL/Exec", {"query": stmt}, st.session_state.token)
                    if exec_result.get("ok"):
                        st.success(exec_result.get("message", "OK"))
                    else:
                        st.error(exec_result.get("message", exec_result.get("error", "failed")))
    else:
        st.info("Login to execute converted QQL against the gateway")


def _default_json() -> str:
    return json.dumps({
        "points": [
            {"id": 1, "payload": {"text": "hello", "category": "test"}},
            {"id": 2, "payload": {"text": "world", "category": "test"}},
        ]
    }, indent=2)
