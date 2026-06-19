"""Convert tab — Qdrant REST JSON → QQL with 110 categorized examples."""

import json
import httpx
import streamlit as st

from utils import GW_URL, api_post, load_payload_examples, decode_data, embed_text, QDRANT_URL

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


def on_example_change():
    """Callback when selected example changes, immediately populating the editor."""
    selected = st.session_state.convert_example_select
    if selected != "— paste custom JSON —" and "payload_examples_dict" in st.session_state:
        raw_val = st.session_state.payload_examples_dict.get(selected, "")
        if raw_val:
            try:
                formatted_val = json.dumps(json.loads(raw_val), indent=2)
                st.session_state.convert_json_input = formatted_val
            except Exception:
                st.session_state.convert_json_input = raw_val


def render():
    st.header("🔄 REST JSON → QQL Converter (Direct)")
    st.markdown("""
    Translate raw Qdrant REST API payloads into QQL statements. This is a **free utility** — no authentication required for conversion.
    
    **Note:** If your JSON payload includes a path (e.g. `{"method": "POST", "path": "/collections/docs/points/query", ...}`), the collection name is extracted from it. Otherwise, specify the collection name below.
    """)

    # Cache payload examples in session state
    if "payload_examples_dict" not in st.session_state:
        st.session_state.payload_examples_dict = load_payload_examples()

    payload_examples = st.session_state.payload_examples_dict

    # Set default JSON input if not present
    if "convert_json_input" not in st.session_state:
        st.session_state.convert_json_input = _default_json()

    # ── Category & Example picker ─────────────────────────────────────
    st.subheader("1. Select/Load Example Payload")
    col_cat, col_ex = st.columns(2)
    
    with col_cat:
        category = st.selectbox(
            "Category", 
            options=list(CATEGORIES.keys()), 
            key="convert_category"
        )
        
    with col_ex:
        patterns = CATEGORIES[category]
        excludes = _EXCLUDE.get(category, [])
        example_names = [
            k for k in payload_examples
            if any(p in k.lower() for p in patterns)
            and not any(e in k.lower() for e in excludes)
        ]
        st.selectbox(
            "Example list",
            options=["— paste custom JSON —"] + example_names,
            key="convert_example_select",
            on_change=on_example_change
        )

    st.divider()

    # ── Main conversion layout ────────────────────────────────────────
    col_left, col_right = st.columns([1, 1])

    with col_left:
        st.subheader("📝 Input: Qdrant REST JSON")
        
        # Check if the JSON is a wrapped request (specifies method + path)
        is_wrapped = False
        try:
            parsed_test = json.loads(st.session_state.convert_json_input)
            if isinstance(parsed_test, dict) and "method" in parsed_test and "path" in parsed_test:
                is_wrapped = True
        except Exception:
            pass

        # Target Collection selector (disabled if path specifies it)
        if is_wrapped:
            st.info("Collection name extracted from the `path` field in your JSON.")
            target_coll = ""
        else:
            target_coll = st.text_input(
                "Target Collection name",
                value="",
                key="convert_target_collection",
                placeholder="e.g. docs, my_collection",
                help="Required for bare JSON payloads. The collection name is used to generate the QQL FROM clause."
            )
            if not target_coll.strip():
                st.warning("Enter a collection name, or use a wrapped payload with `method` and `path` fields.")

        st.text_area(
            "JSON payload editor",
            key="convert_json_input",
            height=350,
            help="Edit the raw Qdrant REST JSON payload here."
        )

        convert_clicked = st.button("🔄 Convert to QQL", type="primary", use_container_width=True)

    with col_right:
        st.subheader("💻 Output: Generated QQL")
        
        # Run conversion when clicked (No JWT token / auth used)
        if convert_clicked:
            source_json = st.session_state.convert_json_input
            if not source_json.strip():
                st.warning("Payload editor is empty.")
                st.session_state.convert_qql_stmts = None
            elif not is_wrapped and not target_coll.strip():
                st.error("Collection name is required for bare payloads. Enter a collection name or use a wrapped payload with `method` and `path`.")
                st.session_state.convert_qql_stmts = None
            else:
                with st.spinner("Converting..."):
                    result = api_post(
                        f"{GW_URL}/qql.QQL/Convert", 
                        {"jsonPayload": source_json, "collection": target_coll}, 
                        token=None
                    )
                
                if not result.get("ok"):
                    st.error(f"Conversion failed: {result.get('error', result.get('message', 'Unknown error'))}")
                    st.session_state.convert_qql_stmts = None
                else:
                    st.session_state.convert_qql_stmts = result.get("statements", [])
                    st.session_state.convert_compare_ran = False
                    st.session_state.convert_rest_res = None
                    st.session_state.convert_qql_res = None

        # Display generated QQL
        qql_stmts = st.session_state.get("convert_qql_stmts")
        if qql_stmts:
            st.success(f"Generated {len(qql_stmts)} QQL statement(s)")
            for stmt in qql_stmts:
                st.code(stmt, language="sql")
                
            st.divider()

            # --- Target Qdrant Endpoint config for execution ---
            st.subheader("⚡ Compare Execution Live (Direct)")
            st.markdown("Verify the translated QQL by running both the original REST JSON and the converted QQL directly against your Qdrant instance.")
            
            qdrant_url = st.text_input(
                "Qdrant REST Endpoint URL",
                value="http://localhost:6333",
                key="convert_qdrant_endpoint",
                help="The direct Qdrant HTTP REST URL (usually port :6333) used to execute direct REST payloads."
            )
            
            compare_btn = st.button("⚡ Run Comparison", type="secondary", use_container_width=True)
            
            if compare_btn:
                st.session_state.convert_compare_ran = True
                source_json = st.session_state.convert_json_input
                
                with st.spinner("Executing direct queries..."):
                    # 1. Run direct REST
                    method, path, body = parse_qdrant_payload(source_json, target_coll)
                    rest_res = run_qdrant_rest(method, path, body, qdrant_url)
                    st.session_state.convert_rest_res = rest_res
                    st.session_state.convert_rest_body = body
                    
                    # 2. Fetch compiled Qdrant request JSON from Gateway Explain (No Auth token)
                    compiled_requests = []
                    for stmt in qql_stmts:
                        explain_res = api_post(f"{GW_URL}/qql.QQL/Explain", {"query": stmt, "json": True}, token=None)
                        if explain_res.get("ok"):
                            compiled_requests.append(explain_res.get("plan"))
                        else:
                            compiled_requests.append(json.dumps({"error": explain_res.get("error", explain_res.get("message", "Explain failed"))}))
                    st.session_state.convert_compiled_requests = compiled_requests
                    
                    # 3. Run QQL directly on Qdrant via Gateway (No Auth token, unfiltered)
                    qql_res = []
                    for stmt in qql_stmts:
                        exec_res = api_post(f"{GW_URL}/qql.QQL/Exec", {"query": stmt}, token=None)
                        qql_res.append(exec_res)
                    st.session_state.convert_qql_res = qql_res

            # Render comparison results if executed
            if st.session_state.get("convert_compare_ran") and st.session_state.convert_rest_res is not None and st.session_state.convert_qql_res is not None:
                rest_resp = st.session_state.convert_rest_res
                qql_resps = st.session_state.convert_qql_res
                last_qql_resp = qql_resps[-1] if qql_resps else {}

                rest_pts = extract_points_from_rest(rest_resp)
                qql_pts = extract_points_from_qql(decode_data(last_qql_resp.get("data")))

                rest_ids = [str(p.get("id")) for p in rest_pts if p.get("id") is not None]
                qql_ids = [str(p.get("id")) for p in qql_pts if p.get("id") is not None]

                # Status check & comparison banner
                is_rest_err = "error" in rest_resp or (isinstance(rest_resp, dict) and rest_resp.get("status", {}).get("error"))
                is_qql_err = not last_qql_resp.get("ok")
                
                if is_rest_err:
                    st.error("❌ Qdrant Direct returned an error.")
                    err_msg = rest_resp.get("error", rest_resp.get("status", {}).get("error", "Unknown error"))
                    
                    if "InferenceService" in err_msg or "Inference" in err_msg:
                        st.info("""
                        💡 **Inference Service Explanation:** 
                        The Qdrant REST payload uses model-based text queries (e.g. `{"query": {"text": "..."}}`). 
                        This failed directly on Qdrant because Qdrant requires a server-side Inference Service configured inside its config.
                        
                        **However, the QQL Gateway manages this automatically!** 
                        The gateway intercepts the query, computes the vector embedding on the fly using its model configuration, and sends the resolved vectors to Qdrant. 
                        This is why the QQL Gateway path succeeds even when raw Qdrant REST fails!
                        """)
                
                if not rest_ids and not qql_ids:
                    qql_ok = all(resp.get("ok") for resp in qql_resps)
                    rest_ok = not is_rest_err
                    if qql_ok and rest_ok:
                        st.success("✅ Success Match: Both endpoints executed successfully.")
                    elif is_qql_err:
                        st.error(f"Gateway execution error: {last_qql_resp.get('message')}")
                else:
                    # Query comparison
                    ids_identical = (rest_ids == qql_ids)
                    scores_matching = scores_match(rest_pts, qql_pts)
                    
                    if ids_identical and scores_matching:
                        st.success("🎉 Perfect Match! Both executions returned the exact same document IDs and similarity scores.")
                    elif ids_identical:
                        st.info("ℹ— Identical IDs returned. Similarity scores differ slightly.")
                    elif set(rest_ids) == set(qql_ids):
                        st.warning("⚠️ Same Documents returned, but ordering is different.")
                    else:
                        st.error("❌ Output Mismatch: The returned documents differ.")

                # Side-by-side Request Payload Comparison
                st.markdown("### 📋 Request Payload translation verification")
                st.caption("Verify that the QQL query compiles to the exact same Qdrant REST JSON representation as the original input payload, confirming that the QQL translation is lossless.")
                
                col_req_rest, col_req_qql = st.columns(2)
                with col_req_rest:
                    st.markdown("**Original REST JSON**")
                    st.caption("Sent directly to Qdrant REST endpoint")
                    st.json(st.session_state.get("convert_rest_body", {}))
                    
                with col_req_qql:
                    st.markdown("**Compiled request JSON generated from QQL**")
                    st.caption("Compiled from generated QQL (Lossless check)")
                    compiled_reqs = st.session_state.get("convert_compiled_requests", [])
                    for idx, req_json in enumerate(compiled_reqs):
                        if len(compiled_reqs) > 1:
                            st.caption(f"Statement {idx + 1}")
                        try:
                            st.json(json.loads(req_json))
                        except Exception:
                            st.code(req_json, language="json")
                
                st.divider()

                # Side-by-side output view
                col_sub_rest, col_sub_qql = st.columns(2)
                
                with col_sub_rest:
                    st.markdown("**Direct Qdrant REST Output**")
                    st.caption(f"Hits direct REST port on {qdrant_url}")
                    if rest_ids:
                        st.metric("Points Returned", len(rest_ids))
                        st.write("IDs:", rest_ids)
                    st.json(rest_resp)

                with col_sub_qql:
                    st.markdown("**QQL Execution Output**")
                    st.caption("Hits Gateway Port `:50051` directly (unfiltered)")
                    if qql_ids:
                        st.metric("Points Returned", len(qql_ids))
                        st.write("IDs:", qql_ids)
                    
                    decoded = decode_data(last_qql_resp.get("data"))
                    st.json({
                        "ok": last_qql_resp.get("ok"),
                        "operation": last_qql_resp.get("operation"),
                        "message": last_qql_resp.get("message"),
                        "data": decoded
                    })
        else:
            st.info("Load or edit a JSON payload and click **Convert to QQL**.")


def parse_qdrant_payload(raw_json: str, target_collection: str) -> tuple[str, str, dict]:
    """Parses Qdrant JSON payload and resolves direct REST endpoint."""
    try:
        data = json.loads(raw_json)
    except Exception:
        return "POST", f"collections/{target_collection}/points/query", {}

    # Check if wrapped (defines method, path, body)
    if isinstance(data, dict) and "method" in data and "path" in data:
        body = data.get("body", data.get("request", {}))
        return data["method"], data["path"], body

    # Infer endpoint based on structure
    method = "POST"
    path = f"collections/{target_collection}/points/query"
    body = data

    if not isinstance(data, dict):
        return method, path, body

    if "points" in data:
        if any(k in data for k in ["ids", "filter"]) and not any(k in data for k in ["points"]):
            method = "POST"
            path = f"collections/{target_collection}/points/delete"
        else:
            method = "PUT"
            path = f"collections/{target_collection}/points"
    elif "ids" in data:
        method = "POST"
        path = f"collections/{target_collection}/points"
    elif "filter" in data and not any(k in data for k in ["vector", "query", "prefetch"]):
        method = "POST"
        path = f"collections/{target_collection}/points/scroll"
    elif "vectors" in data and not any(k in data for k in ["points", "query"]):
        method = "PUT"
        path = f"collections/{target_collection}"
    elif "field_name" in data:
        method = "PUT"
        path = f"collections/{target_collection}/index"
    elif "searches" in data:
        method = "POST"
        path = f"collections/{target_collection}/points/query/batch"
    elif "positive" in data:
        method = "POST"
        path = f"collections/{target_collection}/points/recommend"
    elif "target" in data:
        method = "POST"
        path = f"collections/{target_collection}/points/discover"

    return method, path, body


def run_qdrant_rest(method: str, path: str, body: dict, qdrant_url: str = "http://localhost:6333") -> dict:
    """Sends a request directly to Qdrant REST API.
    
    If the payload contains text queries, computes embeddings first using the local
    embedding service, since Qdrant doesn't have inference configured.
    """
    body = _resolve_text_to_vectors(body)
    url = f"{qdrant_url.rstrip('/')}/{path.lstrip('/')}"
    try:
        if method == "GET":
            resp = httpx.get(url, timeout=10)
        elif method == "POST":
            resp = httpx.post(url, json=body, timeout=10)
        elif method == "PUT":
            resp = httpx.put(url, json=body, timeout=10)
        elif method == "DELETE":
            resp = httpx.delete(url, json=body, timeout=10)
        else:
            return {"error": f"Unsupported method: {method}"}
        return resp.json()
    except Exception as e:
        return {"error": f"Failed to connect to Qdrant REST API at {url}: {str(e)}"}


def _resolve_text_to_vectors(body: dict) -> dict:
    """Replace text queries with computed embeddings for direct Qdrant REST calls."""
    if not isinstance(body, dict):
        return body
    body = json.loads(json.dumps(body))  # deep copy
    _resolve_query_text(body)
    return body


def _resolve_query_text(node: dict):
    """Recursively resolve text queries to vectors."""
    if not isinstance(node, dict):
        return
    
    # Handle {"query": {"text": "...", "model": "..."}} -> {"query": [...vector...]}
    query = node.get("query")
    if isinstance(query, dict) and "text" in query and "model" in query:
        text = query["text"]
        model = query["model"]
        if "fusion" not in query:  # skip fusion nodes
            vec = embed_text(text, model)
            if vec:
                node["query"] = vec
    
    # Handle prefetch arrays
    prefetch = node.get("prefetch")
    if isinstance(prefetch, list):
        for item in prefetch:
            if isinstance(item, dict):
                _resolve_query_text(item)


def extract_points_from_rest(resp: dict) -> list[dict]:
    if not isinstance(resp, dict):
        return []
    res = resp.get("result", [])
    if isinstance(res, dict):
        return res.get("points", [])
    if isinstance(res, list):
        return res
    return []


def extract_points_from_qql(qql_data: any) -> list[dict]:
    if isinstance(qql_data, list):
        return qql_data
    if isinstance(qql_data, dict):
        return qql_data.get("points", qql_data.get("result", []))
    return []


def scores_match(rest_pts, qql_pts) -> bool:
    if len(rest_pts) != len(qql_pts):
        return False
    for r, q in zip(rest_pts, qql_pts):
        rs = r.get("score")
        qs = q.get("score")
        if rs is None or qs is None:
            if rs != qs:
                return False
        else:
            if abs(float(rs) - float(qs)) > 1e-4:
                return False
    return True


def _default_json() -> str:
    return json.dumps({
        "points": [
            {"id": 1, "payload": {"text": "hello", "category": "test"}},
            {"id": 2, "payload": {"text": "world", "category": "test"}},
        ]
    }, indent=2)

