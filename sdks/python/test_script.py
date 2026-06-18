"""Test script for qql_intercept."""
from qdrant_client import QdrantClient, models

client = QdrantClient(url="http://localhost:6333")

client.create_collection(
    collection_name="docs",
    vectors_config={
        "dense": models.VectorParams(size=384, distance=models.Distance.COSINE),
        "colbert": models.VectorParams(
            size=128,
            distance=models.Distance.COSINE,
            multivector_config=models.MultiVectorConfig(comparator=models.MultiVectorComparator.MAX_SIM),
            hnsw_config=models.HnswConfigDiff(m=0)
        )
    }
)

client.upsert(
    collection_name="docs",
    points=[
        models.PointStruct(id=1, vector={"dense": [0.1, 0.2, 0.3]}, payload={"title": "hello"}),
    ]
)

results = client.query_points(
    collection_name="docs",
    query=[0.1, 0.2, 0.3],
    prefetch=[
        models.Prefetch(query=[0.1, 0.2, 0.3], using="dense", limit=50),
        models.Prefetch(query=[0.1, 0.2, 0.3], using="colbert", limit=50),
    ],
    using="colbert",
    limit=10,
    with_payload=True
)
