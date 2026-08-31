package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var _ Repository = (*MemoryRepository)(nil)
var _ Searcher = (*MemoryRepository)(nil)
var _ CurrentRevisionSetter = (*MemoryRepository)(nil)
var _ SourceLister = (*MemoryRepository)(nil)
var _ SourceDetailReader = (*MemoryRepository)(nil)
var _ ManualImportIdempotencyRepository = (*MemoryRepository)(nil)
var _ ManualImportRepository = (*MemoryRepository)(nil)

// MemoryRepository is the deterministic local implementation used when the
// backend starts without DATABASE_URL. It keeps the same workspace boundary
// as the PostgreSQL adapter and is intentionally process-local, not a hidden
// production fallback.
type MemoryRepository struct {
	mu            sync.RWMutex
	sources       map[scopedID]KnowledgeSource
	items         map[scopedID]SourceItem
	revisions     map[scopedID]SourceRevision
	chunks        map[scopedID]KnowledgeChunk
	topics        map[scopedID]Topic
	claims        map[scopedID]KnowledgeClaim
	evidence      map[scopedID][]ClaimEvidence
	manualImports map[manualImportKey]ManualImportIdempotencyRecord
}

type scopedID struct {
	workspaceID string
	id          string
}

type manualImportKey struct {
	workspaceID    string
	userID         string
	operation      string
	idempotencyKey string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		sources:       make(map[scopedID]KnowledgeSource),
		items:         make(map[scopedID]SourceItem),
		revisions:     make(map[scopedID]SourceRevision),
		chunks:        make(map[scopedID]KnowledgeChunk),
		topics:        make(map[scopedID]Topic),
		claims:        make(map[scopedID]KnowledgeClaim),
		evidence:      make(map[scopedID][]ClaimEvidence),
		manualImports: make(map[manualImportKey]ManualImportIdempotencyRecord),
	}
}

func (r *MemoryRepository) LookupManualImportIdempotency(_ context.Context, scope WorkspaceScope, userID, idempotencyKey string) (ManualImportIdempotencyRecord, bool, error) {
	if err := scope.Validate(); err != nil {
		return ManualImportIdempotencyRecord{}, false, err
	}
	if err := requireIdentifier("userId", userID); err != nil {
		return ManualImportIdempotencyRecord{}, false, err
	}
	if err := requireIdentifier("idempotencyKey", idempotencyKey); err != nil {
		return ManualImportIdempotencyRecord{}, false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.manualImports[manualImportKey{workspaceID: scope.ID, userID: userID, operation: "knowledge.import_manual", idempotencyKey: idempotencyKey}]
	if !ok {
		return ManualImportIdempotencyRecord{}, false, nil
	}
	return cloneManualImportIdempotency(record), true, nil
}

func (r *MemoryRepository) SaveManualImportIdempotency(_ context.Context, scope WorkspaceScope, record ManualImportIdempotencyRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if record.WorkspaceID != scope.ID {
		return ErrWorkspaceMismatch
	}
	for field, value := range map[string]string{
		"userId": record.UserID, "operation": record.Operation,
		"idempotencyKey": record.IdempotencyKey, "requestHash": record.RequestHash,
	} {
		if err := requireIdentifier(field, value); err != nil {
			return err
		}
	}
	if record.Operation != "knowledge.import_manual" || len(record.ResponseJSON) == 0 {
		return ErrInvalidInput
	}
	key := manualImportKey{workspaceID: scope.ID, userID: record.UserID, operation: record.Operation, idempotencyKey: record.IdempotencyKey}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.manualImports[key]; exists {
		return ErrConflict
	}
	r.manualImports[key] = cloneManualImportIdempotency(record)
	return nil
}

func cloneManualImportIdempotency(record ManualImportIdempotencyRecord) ManualImportIdempotencyRecord {
	record.ResponseJSON = append([]byte(nil), record.ResponseJSON...)
	return record
}

// CommitManualImport keeps the development repository's idempotency and
// source graph under one mutex. This mirrors the PostgreSQL transaction path
// closely enough that concurrent keyed imports exercise the same contract in
// unit tests without making the memory adapter pretend to be durable.
func (r *MemoryRepository) CommitManualImport(_ context.Context, scope WorkspaceScope, request ManualImportCommitRequest) (ManualImportCommit, error) {
	if err := validateManualImportCommitRequest(scope, request); err != nil {
		return ManualImportCommit{}, err
	}
	key := manualImportKey{
		workspaceID: scope.ID, userID: request.UserID,
		operation: manualImportOperationName, idempotencyKey: request.IdempotencyKey,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if record, exists := r.manualImports[key]; exists {
		return ManualImportCommit{Record: cloneManualImportIdempotency(record), Replayed: true}, nil
	}

	bundle := request.Bundle
	sourceKey := scopedID{scope.ID, bundle.Source.ID}
	effectiveSource, sourceExists := r.sources[sourceKey]
	if sourceExists {
		if effectiveSource.DeletedAt != nil {
			return ManualImportCommit{}, ErrNotFound
		}
	} else {
		effectiveSource = cloneSource(bundle.Source)
	}

	itemKey := scopedID{scope.ID, bundle.Item.ID}
	effectiveItem, itemExists := r.items[itemKey]
	if itemExists {
		if effectiveItem.DeletedAt != nil {
			return ManualImportCommit{}, ErrNotFound
		}
		if effectiveItem.SourceID != effectiveSource.ID {
			return ManualImportCommit{}, ErrConflict
		}
	} else {
		effectiveItem = cloneSourceItem(bundle.Item)
	}

	revisionKey := scopedID{scope.ID, bundle.Revision.ID}
	effectiveRevision, revisionExists := r.revisions[revisionKey]
	if revisionExists {
		if !sameManualImportRevision(effectiveRevision, bundle.Revision) {
			return ManualImportCommit{}, ErrRevisionImmutable
		}
	} else {
		effectiveRevision = cloneRevision(bundle.Revision)
	}

	chunkKey := scopedID{scope.ID, bundle.Chunk.ID}
	effectiveChunk, chunkExists := r.chunks[chunkKey]
	if chunkExists && !sameManualImportChunk(effectiveChunk, bundle.Chunk) {
		return ManualImportCommit{}, ErrConflict
	}

	if !sourceExists {
		r.sources[sourceKey] = cloneSource(effectiveSource)
	}
	if !itemExists {
		r.items[itemKey] = cloneSourceItem(effectiveItem)
	}
	if !revisionExists {
		r.revisions[revisionKey] = cloneRevision(effectiveRevision)
	}
	if !chunkExists {
		r.chunks[chunkKey] = cloneChunk(bundle.Chunk)
	}
	if effectiveItem.CurrentRevisionID != effectiveRevision.ID {
		effectiveItem.CurrentRevisionID = effectiveRevision.ID
		effectiveItem.Version++
		effectiveItem.UpdatedAt = effectiveRevision.IngestedAt
		r.items[itemKey] = cloneSourceItem(effectiveItem)
	}

	projection := manualImportProjection(effectiveSource, effectiveRevision.ID, bundle.Chunk.ID)
	responseJSON, err := json.Marshal(projection)
	if err != nil {
		return ManualImportCommit{}, fmt.Errorf("encode manual import replay: %w", err)
	}
	record := ManualImportIdempotencyRecord{
		WorkspaceID: scope.ID, UserID: request.UserID,
		Operation: manualImportOperationName, IdempotencyKey: request.IdempotencyKey,
		RequestHash: request.RequestHash, ResponseJSON: responseJSON,
	}
	r.manualImports[key] = cloneManualImportIdempotency(record)
	return ManualImportCommit{Projection: projection}, nil
}

func (r *MemoryRepository) CreateSource(_ context.Context, scope WorkspaceScope, source KnowledgeSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemoryScope(scope, source.WorkspaceID, source.Validate); err != nil {
		return err
	}
	key := scopedID{scope.ID, source.ID}
	if _, exists := r.sources[key]; exists {
		return ErrConflict
	}
	r.sources[key] = cloneSource(source)
	return nil
}

func (r *MemoryRepository) GetSource(_ context.Context, scope WorkspaceScope, id string) (KnowledgeSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	source, ok := r.sources[scopedID{scope.ID, id}]
	if !ok || source.DeletedAt != nil {
		return KnowledgeSource{}, ErrNotFound
	}
	return cloneSource(source), nil
}

func (r *MemoryRepository) ListSources(_ context.Context, scope WorkspaceScope, limit int) ([]KnowledgeSource, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]KnowledgeSource, 0)
	for _, source := range r.sources {
		if source.WorkspaceID != scope.ID || source.DeletedAt != nil {
			continue
		}
		result = append(result, cloneSource(source))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *MemoryRepository) ListSourceItems(_ context.Context, scope WorkspaceScope, sourceID string, limit int) ([]SourceItem, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetSource(context.Background(), scope, sourceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]SourceItem, 0)
	for _, item := range r.items {
		if item.WorkspaceID == scope.ID && item.SourceID == sourceID && item.DeletedAt == nil {
			items = append(items, cloneSourceItem(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemoryRepository) ListRevisions(_ context.Context, scope WorkspaceScope, itemID string, limit int) ([]SourceRevision, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetSourceItem(context.Background(), scope, itemID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	revisions := make([]SourceRevision, 0)
	for _, revision := range r.revisions {
		if revision.WorkspaceID == scope.ID && revision.SourceItemID == itemID {
			revisions = append(revisions, cloneRevision(revision))
		}
	}
	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].IngestedAt.Equal(revisions[j].IngestedAt) {
			return revisions[i].ID < revisions[j].ID
		}
		return revisions[i].IngestedAt.After(revisions[j].IngestedAt)
	})
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}
	return revisions, nil
}

func (r *MemoryRepository) ListChunks(_ context.Context, scope WorkspaceScope, revisionID string, limit int) ([]KnowledgeChunk, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetRevision(context.Background(), scope, revisionID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	chunks := make([]KnowledgeChunk, 0)
	for _, chunk := range r.chunks {
		if chunk.WorkspaceID == scope.ID && chunk.RevisionID == revisionID {
			chunks = append(chunks, cloneChunk(chunk))
		}
	}
	sort.Slice(chunks, func(i, j int) bool {
		if chunks[i].Ordinal == chunks[j].Ordinal {
			return chunks[i].ID < chunks[j].ID
		}
		return chunks[i].Ordinal < chunks[j].Ordinal
	})
	if len(chunks) > limit {
		chunks = chunks[:limit]
	}
	return chunks, nil
}

func (r *MemoryRepository) ListEvidenceForSource(_ context.Context, scope WorkspaceScope, sourceID string, limit int) ([]ClaimEvidence, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, err := r.GetSource(context.Background(), scope, sourceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	itemIDs := make(map[string]struct{})
	for _, item := range r.items {
		if item.WorkspaceID == scope.ID && item.SourceID == sourceID {
			itemIDs[item.ID] = struct{}{}
		}
	}
	revisionIDs := make(map[string]struct{})
	for _, revision := range r.revisions {
		if revision.WorkspaceID == scope.ID {
			if _, ok := itemIDs[revision.SourceItemID]; ok {
				revisionIDs[revision.ID] = struct{}{}
			}
		}
	}
	evidence := make([]ClaimEvidence, 0)
	for _, items := range r.evidence {
		for _, item := range items {
			if item.WorkspaceID == scope.ID {
				if _, ok := revisionIDs[item.SourceRevisionID]; ok {
					evidence = append(evidence, item)
				}
			}
		}
	}
	sort.Slice(evidence, func(i, j int) bool {
		if evidence[i].CreatedAt.Equal(evidence[j].CreatedAt) {
			return evidence[i].ID < evidence[j].ID
		}
		return evidence[i].CreatedAt.Before(evidence[j].CreatedAt)
	})
	if len(evidence) > limit {
		evidence = evidence[:limit]
	}
	return cloneEvidence(evidence), nil
}

func (r *MemoryRepository) CreateSourceItem(_ context.Context, scope WorkspaceScope, item SourceItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemoryScope(scope, item.WorkspaceID, item.Validate); err != nil {
		return err
	}
	if _, exists := r.sources[scopedID{scope.ID, item.SourceID}]; !exists {
		return ErrNotFound
	}
	key := scopedID{scope.ID, item.ID}
	if _, exists := r.items[key]; exists {
		return ErrConflict
	}
	r.items[key] = cloneSourceItem(item)
	return nil
}

func (r *MemoryRepository) GetSourceItem(_ context.Context, scope WorkspaceScope, id string) (SourceItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[scopedID{scope.ID, id}]
	if !ok || item.DeletedAt != nil {
		return SourceItem{}, ErrNotFound
	}
	return cloneSourceItem(item), nil
}

func (r *MemoryRepository) CreateRevision(_ context.Context, scope WorkspaceScope, revision SourceRevision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemoryScope(scope, revision.WorkspaceID, revision.Validate); err != nil {
		return err
	}
	if _, exists := r.items[scopedID{scope.ID, revision.SourceItemID}]; !exists {
		return ErrNotFound
	}
	key := scopedID{scope.ID, revision.ID}
	if existing, exists := r.revisions[key]; exists {
		if err := AssertRevisionImmutable(existing, revision); err != nil {
			return err
		}
		return ErrConflict
	}
	r.revisions[key] = cloneRevision(revision)
	return nil
}

func (r *MemoryRepository) GetRevision(_ context.Context, scope WorkspaceScope, id string) (SourceRevision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	revision, ok := r.revisions[scopedID{scope.ID, id}]
	if !ok {
		return SourceRevision{}, ErrNotFound
	}
	return cloneRevision(revision), nil
}

func (r *MemoryRepository) SetCurrentRevision(_ context.Context, scope WorkspaceScope, itemID, revisionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[scopedID{scope.ID, itemID}]
	if !ok || item.DeletedAt != nil {
		return ErrNotFound
	}
	revision, ok := r.revisions[scopedID{scope.ID, revisionID}]
	if !ok || revision.SourceItemID != itemID {
		return ErrNotFound
	}
	if item.CurrentRevisionID == revisionID {
		return nil
	}
	item.CurrentRevisionID = revisionID
	item.Version++
	item.UpdatedAt = revision.IngestedAt
	r.items[scopedID{scope.ID, itemID}] = cloneSourceItem(item)
	return nil
}

func (r *MemoryRepository) CreateChunk(_ context.Context, scope WorkspaceScope, chunk KnowledgeChunk) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemoryScope(scope, chunk.WorkspaceID, chunk.Validate); err != nil {
		return err
	}
	if _, exists := r.revisions[scopedID{scope.ID, chunk.RevisionID}]; !exists {
		return ErrNotFound
	}
	key := scopedID{scope.ID, chunk.ID}
	if _, exists := r.chunks[key]; exists {
		return ErrConflict
	}
	r.chunks[key] = cloneChunk(chunk)
	return nil
}

func (r *MemoryRepository) GetChunk(_ context.Context, scope WorkspaceScope, id string) (KnowledgeChunk, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	chunk, ok := r.chunks[scopedID{scope.ID, id}]
	if !ok {
		return KnowledgeChunk{}, ErrNotFound
	}
	return cloneChunk(chunk), nil
}

func (r *MemoryRepository) CreateTopic(_ context.Context, scope WorkspaceScope, topic Topic) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemoryScope(scope, topic.WorkspaceID, topic.Validate); err != nil {
		return err
	}
	if topic.ParentID != "" {
		parent, exists := r.topics[scopedID{scope.ID, topic.ParentID}]
		if !exists || parent.DeletedAt != nil {
			return ErrNotFound
		}
	}
	key := scopedID{scope.ID, topic.ID}
	if _, exists := r.topics[key]; exists {
		return ErrConflict
	}
	r.topics[key] = cloneTopic(topic)
	return nil
}

func (r *MemoryRepository) GetTopic(_ context.Context, scope WorkspaceScope, id string) (Topic, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	topic, ok := r.topics[scopedID{scope.ID, id}]
	if !ok || topic.DeletedAt != nil {
		return Topic{}, ErrNotFound
	}
	return cloneTopic(topic), nil
}

func (r *MemoryRepository) CreateClaimBundle(_ context.Context, scope WorkspaceScope, claim KnowledgeClaim, evidence []ClaimEvidence) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateMemoryScope(scope, claim.WorkspaceID, claim.Validate); err != nil {
		return err
	}
	if err := ValidateClaimBundle(claim, evidence); err != nil {
		return err
	}
	if claim.TopicID != "" {
		topic, exists := r.topics[scopedID{scope.ID, claim.TopicID}]
		if !exists || topic.DeletedAt != nil {
			return ErrNotFound
		}
	}
	for _, item := range evidence {
		revision, exists := r.revisions[scopedID{scope.ID, item.SourceRevisionID}]
		if !exists {
			return ErrNotFound
		}
		if item.ChunkID != "" {
			chunk, exists := r.chunks[scopedID{scope.ID, item.ChunkID}]
			if !exists || chunk.RevisionID != revision.ID {
				return ErrNotFound
			}
		}
	}
	key := scopedID{scope.ID, claim.ID}
	if _, exists := r.claims[key]; exists {
		return ErrConflict
	}
	r.claims[key] = cloneClaim(claim)
	r.evidence[key] = cloneEvidence(evidence)
	return nil
}

func (r *MemoryRepository) GetClaim(_ context.Context, scope WorkspaceScope, id string) (KnowledgeClaim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	claim, ok := r.claims[scopedID{scope.ID, id}]
	if !ok || claim.DeletedAt != nil {
		return KnowledgeClaim{}, ErrNotFound
	}
	return cloneClaim(claim), nil
}

func (r *MemoryRepository) ListClaimEvidence(_ context.Context, scope WorkspaceScope, claimID string) ([]ClaimEvidence, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items, ok := r.evidence[scopedID{scope.ID, claimID}]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneEvidence(items), nil
}

func (r *MemoryRepository) Search(_ context.Context, scope WorkspaceScope, query string, limit int) ([]SearchResult, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return []SearchResult{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	type scored struct {
		result SearchResult
		score  int
	}
	items := make([]scored, 0)
	for _, item := range r.items {
		if item.WorkspaceID != scope.ID || item.DeletedAt != nil || item.CurrentRevisionID == "" {
			continue
		}
		source := r.sources[scopedID{scope.ID, item.SourceID}]
		revision, ok := r.revisions[scopedID{scope.ID, item.CurrentRevisionID}]
		if !ok {
			continue
		}
		for _, chunk := range r.chunks {
			if chunk.WorkspaceID != scope.ID || chunk.RevisionID != revision.ID {
				continue
			}
			score := countTermMatches(terms, strings.Join([]string{source.Name, source.URI, item.Title, item.URI, revision.Content, chunk.Text}, " "))
			if score == 0 {
				continue
			}
			items = append(items, scored{result: SearchResult{Source: cloneSource(source), Item: cloneSourceItem(item), Revision: cloneRevision(revision), Chunk: cloneChunk(chunk)}, score: score})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		left, right := items[i].result, items[j].result
		if left.Source.ID != right.Source.ID {
			return left.Source.ID < right.Source.ID
		}
		if left.Chunk.Ordinal != right.Chunk.Ordinal {
			return left.Chunk.Ordinal < right.Chunk.Ordinal
		}
		return left.Chunk.ID < right.Chunk.ID
	})
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]SearchResult, 0, len(items))
	for _, item := range items {
		result = append(result, item.result)
	}
	return result, nil
}

func validateMemoryScope(scope WorkspaceScope, entityWorkspaceID string, validate func() error) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if entityWorkspaceID != scope.ID {
		return ErrWorkspaceMismatch
	}
	return validate()
}

func searchTerms(query string) []string {
	seen := make(map[string]struct{})
	terms := make([]string, 0)
	for _, term := range strings.Fields(strings.ToLower(strings.TrimSpace(query))) {
		if len(term) < 2 {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}

func countTermMatches(terms []string, value string) int {
	lower := strings.ToLower(value)
	score := 0
	for _, term := range terms {
		if strings.Contains(lower, term) {
			score++
		}
	}
	return score
}

func cloneSource(value KnowledgeSource) KnowledgeSource {
	value.Metadata = cloneMap(value.Metadata)
	return value
}

func cloneSourceItem(value SourceItem) SourceItem       { return value }
func cloneRevision(value SourceRevision) SourceRevision { return value }

func cloneChunk(value KnowledgeChunk) KnowledgeChunk {
	value.Embedding = append([]float32(nil), value.Embedding...)
	return value
}

func cloneTopic(value Topic) Topic                   { return value }
func cloneClaim(value KnowledgeClaim) KnowledgeClaim { return value }

func cloneEvidence(value []ClaimEvidence) []ClaimEvidence {
	return append([]ClaimEvidence(nil), value...)
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
