package commands

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/output"
)

func captureCommandStreams(t *testing.T, fn func(*output.Outputter)) (string, string) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	fn(output.NewOutputterWithWriters(stdout, stderr))
	return stdout.String(), stderr.String()
}

func captureCommandResult(t *testing.T, fn func(*output.Outputter) error) (string, string, error) {
	t.Helper()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := fn(output.NewOutputterWithWriters(stdout, stderr))
	return stdout.String(), stderr.String(), err
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, context.DeadlineExceeded
}

type fakeQdrantClient struct {
	mu                 sync.Mutex
	exists             bool
	info               *qdrant.CollectionInfo
	createRequests     []*qdrant.CreateCollection
	updateRequests     []*qdrant.UpdateCollection
	fieldIndexRequests []*qdrant.CreateFieldIndexCollection
	upserts            []*qdrant.UpsertPoints
	queryRequests      []*qdrant.QueryPoints
	groupRequests      []*qdrant.QueryPointGroups
	groupResults       []*qdrant.PointGroup
	updateVectors      []*qdrant.UpdatePointVectors
	setPayloads        []*qdrant.SetPayloadPoints
	scrollRecords      []*qdrant.RetrievedPoint
	scrollOffset       *qdrant.PointId
	getRecords         []*qdrant.RetrievedPoint
}

func newFakeQdrantClient() *fakeQdrantClient { return &fakeQdrantClient{} }

func (f *fakeQdrantClient) ListCollections(context.Context) ([]string, error) { return nil, nil }

func (f *fakeQdrantClient) CollectionExists(context.Context, string) (bool, error) {
	return f.exists, nil
}

func (f *fakeQdrantClient) GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error) {
	if f.info == nil {
		return nil, errors.New("missing collection")
	}
	return f.info, nil
}

func (f *fakeQdrantClient) CreateCollection(_ context.Context, req *qdrant.CreateCollection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createRequests = append(f.createRequests, req)
	f.exists = true
	f.info = &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{
			QuantizationConfig: req.QuantizationConfig,
			Params: &qdrant.CollectionParams{
				VectorsConfig:       req.VectorsConfig,
				SparseVectorsConfig: req.SparseVectorsConfig,
			},
		},
	}
	return nil
}

func (f *fakeQdrantClient) UpdateCollection(_ context.Context, req *qdrant.UpdateCollection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateRequests = append(f.updateRequests, req)
	return nil
}

func (f *fakeQdrantClient) DeleteCollection(context.Context, string) error { return nil }

func (f *fakeQdrantClient) Upsert(_ context.Context, req *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts = append(f.upserts, req)
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantClient) Query(_ context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryRequests = append(f.queryRequests, req)
	return nil, nil
}

func (f *fakeQdrantClient) QueryGroups(_ context.Context, req *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupRequests = append(f.groupRequests, req)
	return f.groupResults, nil
}

func (f *fakeQdrantClient) Delete(context.Context, *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantClient) UpdateVectors(_ context.Context, req *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateVectors = append(f.updateVectors, req)
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantClient) SetPayload(_ context.Context, req *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setPayloads = append(f.setPayloads, req)
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantClient) CreateFieldIndex(_ context.Context, req *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fieldIndexRequests = append(f.fieldIndexRequests, req)
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantClient) Count(context.Context, *qdrant.CountPoints) (uint64, error) { return 0, nil }

func (f *fakeQdrantClient) ScrollAndOffset(context.Context, *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error) {
	return f.scrollRecords, f.scrollOffset, nil
}

func (f *fakeQdrantClient) Get(context.Context, *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
	return f.getRecords, nil
}
