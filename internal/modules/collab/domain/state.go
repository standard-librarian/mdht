package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrWorkspaceNotInitialized = errors.New("workspace is not initialized")
	ErrInvalidWorkspaceName    = errors.New("workspace name is invalid")
	ErrInvalidPeerAddress      = errors.New("peer address is invalid")
	ErrPeerNotFound            = errors.New("peer not found")
	ErrPeerNotApproved         = errors.New("peer is not approved")
	ErrInvalidAuthTag          = errors.New("invalid auth tag")
	ErrWorkspaceMismatch       = errors.New("workspace mismatch")
	ErrUnknownOpKind           = errors.New("unknown op kind")
	ErrDaemonNotRunning        = errors.New("collab daemon is not running")
	ErrDaemonStartFailed       = errors.New("collab daemon start failed")
	ErrConflictNotFound        = errors.New("conflict not found")
	ErrConflictStrategyInvalid = errors.New("conflict strategy is invalid")
	ErrMigrationFailed         = errors.New("collab migration failed")
)

const (
	SchemaVersionV2 = 2
)

type ErrorCode string

const (
	ErrorWorkspaceUninitialized ErrorCode = "ERR_WORKSPACE_UNINITIALIZED"
	ErrorPeerNotApproved        ErrorCode = "ERR_PEER_NOT_APPROVED"
	ErrorAuthInvalid            ErrorCode = "ERR_AUTH_INVALID"
	ErrorWorkspaceMismatch      ErrorCode = "ERR_WORKSPACE_MISMATCH"
	ErrorConflictNotFound       ErrorCode = "ERR_CONFLICT_NOT_FOUND"
	ErrorConflictStrategy       ErrorCode = "ERR_CONFLICT_STRATEGY_INVALID"
	ErrorDaemonNotRunning       ErrorCode = "ERR_DAEMON_NOT_RUNNING"
	ErrorMigrationFailed        ErrorCode = "ERR_MIGRATION_FAILED"
)

type CodedError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e CodedError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Message
}

func NewCodedError(code ErrorCode, message string) error {
	return CodedError{Code: code, Message: message}
}

func ErrorCodeFromError(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var coded CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	switch {
	case errors.Is(err, ErrWorkspaceNotInitialized):
		return ErrorWorkspaceUninitialized
	case errors.Is(err, ErrPeerNotApproved):
		return ErrorPeerNotApproved
	case errors.Is(err, ErrInvalidAuthTag):
		return ErrorAuthInvalid
	case errors.Is(err, ErrWorkspaceMismatch):
		return ErrorWorkspaceMismatch
	case errors.Is(err, ErrConflictNotFound):
		return ErrorConflictNotFound
	case errors.Is(err, ErrConflictStrategyInvalid):
		return ErrorConflictStrategy
	case errors.Is(err, ErrDaemonNotRunning):
		return ErrorDaemonNotRunning
	case errors.Is(err, ErrMigrationFailed):
		return ErrorMigrationFailed
	default:
		return ""
	}
}

type OpKind string

const (
	OpKindPutRegister   OpKind = "put_register"
	OpKindAddSet        OpKind = "add_set"
	OpKindRemoveSet     OpKind = "remove_set"
	OpKindInsertSeq     OpKind = "insert_seq"
	OpKindDeleteSeq     OpKind = "delete_seq"
	OpKindTombstone     OpKind = "tombstone_entity"
	OpKindReconcileHint OpKind = "reconcile_hint"
)

func (k OpKind) Validate() error {
	switch k {
	case OpKindPutRegister, OpKindAddSet, OpKindRemoveSet, OpKindInsertSeq, OpKindDeleteSeq, OpKindTombstone, OpKindReconcileHint:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnknownOpKind, k)
	}
}

type Workspace struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	SchemaVersion int       `json:"schema_version"`
}

func (w Workspace) Validate() error {
	if strings.TrimSpace(w.Name) == "" {
		return ErrInvalidWorkspaceName
	}
	if strings.TrimSpace(w.ID) == "" {
		return fmt.Errorf("workspace id is required")
	}
	if w.SchemaVersion == 0 {
		w.SchemaVersion = SchemaVersionV2
	}
	if w.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("unsupported workspace schema version: %d", w.SchemaVersion)
	}
	return nil
}

type NodeIdentity struct {
	NodeID     string `json:"node_id"`
	PrivateKey string `json:"private_key"`
}

type Peer struct {
	PeerID    string    `json:"peer_id"`
	Address   string    `json:"address"`
	Label     string    `json:"label,omitempty"`
	State     PeerState `json:"state"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	AddedAt   time.Time `json:"added_at"`
	LastError string    `json:"last_error,omitempty"`
}

type PeerState string

const (
	PeerStatePending  PeerState = "pending"
	PeerStateApproved PeerState = "approved"
	PeerStateRevoked  PeerState = "revoked"
)

func (p Peer) IsApproved() bool {
	return p.State == PeerStateApproved
}

type KeyRecord struct {
	ID        string    `json:"id"`
	KeyBase64 string    `json:"key_base64"`
	CreatedAt time.Time `json:"created_at"`
}

type KeyRing struct {
	ActiveKeyID string      `json:"active_key_id"`
	Keys        []KeyRecord `json:"keys"`
	GraceUntil  time.Time   `json:"grace_until,omitempty"`
}

func (k KeyRing) ResolveKey(keyID string, now time.Time) ([]byte, error) {
	for _, item := range k.Keys {
		if item.ID != keyID {
			continue
		}
		if keyID != k.ActiveKeyID && !k.GraceUntil.IsZero() && now.UTC().After(k.GraceUntil.UTC()) {
			return nil, ErrInvalidAuthTag
		}
		return RandomWorkspaceKey(item.KeyBase64)
	}
	return nil, ErrInvalidAuthTag
}

type ActivityEventType string

const (
	ActivityPeerConnected   ActivityEventType = "peer_connected"
	ActivityPeerRejected    ActivityEventType = "peer_rejected"
	ActivitySyncApplied     ActivityEventType = "sync_applied"
	ActivityConflictCreated ActivityEventType = "conflict_created"
	ActivityConflictSolved  ActivityEventType = "conflict_resolved"
	ActivityKeyRotated      ActivityEventType = "key_rotated"
	ActivityMigration       ActivityEventType = "migration"
)

type ActivityEvent struct {
	ID         string            `json:"id"`
	OccurredAt time.Time         `json:"occurred_at"`
	Type       ActivityEventType `json:"type"`
	Message    string            `json:"message"`
	Fields     map[string]string `json:"fields,omitempty"`
}

type ConflictStatus string

const (
	ConflictStatusOpen     ConflictStatus = "open"
	ConflictStatusResolved ConflictStatus = "resolved"
)

type ConflictStrategy string

const (
	ConflictStrategyLocal  ConflictStrategy = "local"
	ConflictStrategyRemote ConflictStrategy = "remote"
	ConflictStrategyMerge  ConflictStrategy = "merge"
)

func (s ConflictStrategy) Validate() error {
	switch s {
	case ConflictStrategyLocal, ConflictStrategyRemote, ConflictStrategyMerge:
		return nil
	default:
		return ErrConflictStrategyInvalid
	}
}

type ConflictRecord struct {
	ID            string         `json:"id"`
	EntityKey     string         `json:"entity_key"`
	Field         string         `json:"field"`
	LocalValue    string         `json:"local_value"`
	RemoteValue   string         `json:"remote_value"`
	Status        ConflictStatus `json:"status"`
	Strategy      string         `json:"strategy,omitempty"`
	MergedValue   string         `json:"merged_value,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	ResolvedAt    time.Time      `json:"resolved_at,omitempty"`
	SourceNodeID  string         `json:"source_node_id,omitempty"`
	ResolvedBy    string         `json:"resolved_by,omitempty"`
	ResolutionOp  string         `json:"resolution_op,omitempty"`
	WorkspaceID   string         `json:"workspace_id,omitempty"`
	WorkspaceKey  string         `json:"workspace_key_id,omitempty"`
	SchemaVersion int            `json:"schema_version"`
}

type HLC struct {
	Wall    int64  `json:"wall"`
	Counter int64  `json:"counter"`
	NodeID  string `json:"node_id"`
}

func (h HLC) String() string {
	return fmt.Sprintf("%d:%d:%s", h.Wall, h.Counter, h.NodeID)
}

func ParseHLC(raw string) HLC {
	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return HLC{}
	}
	wall, _ := strconv.ParseInt(parts[0], 10, 64)
	counter, _ := strconv.ParseInt(parts[1], 10, 64)
	return HLC{Wall: wall, Counter: counter, NodeID: parts[2]}
}

func CompareHLC(a, b HLC) int {
	if a.Wall < b.Wall {
		return -1
	}
	if a.Wall > b.Wall {
		return 1
	}
	if a.Counter < b.Counter {
		return -1
	}
	if a.Counter > b.Counter {
		return 1
	}
	if a.NodeID < b.NodeID {
		return -1
	}
	if a.NodeID > b.NodeID {
		return 1
	}
	return 0
}

func NextHLC(now time.Time, last HLC, nodeID string) HLC {
	wall := now.UTC().UnixMilli()
	if wall < last.Wall {
		wall = last.Wall
	}
	counter := int64(0)
	if wall == last.Wall {
		counter = last.Counter + 1
	}
	return HLC{Wall: wall, Counter: counter, NodeID: nodeID}
}

type OpEnvelope struct {
	WorkspaceID    string          `json:"workspace_id"`
	WorkspaceKeyID string          `json:"workspace_key_id"`
	NodeID         string          `json:"node_id"`
	PeerID         string          `json:"peer_id,omitempty"`
	EntityKey      string          `json:"entity_key"`
	OpID           string          `json:"op_id"`
	HLCTimestamp   string          `json:"hlc_timestamp"`
	OpKind         OpKind          `json:"op_kind"`
	Payload        json.RawMessage `json:"payload"`
	AuthTag        string          `json:"auth_tag"`
	SchemaVersion  int             `json:"schema_version"`
}

func (o OpEnvelope) Validate() error {
	if o.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(o.WorkspaceKeyID) == "" {
		return fmt.Errorf("workspace_key_id is required")
	}
	if o.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if o.OpID == "" {
		return fmt.Errorf("op_id is required")
	}
	if o.EntityKey == "" {
		return fmt.Errorf("entity_key is required")
	}
	if err := o.OpKind.Validate(); err != nil {
		return err
	}
	if o.HLCTimestamp == "" {
		return fmt.Errorf("hlc_timestamp is required")
	}
	if o.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("unsupported schema version: %d", o.SchemaVersion)
	}
	return nil
}

type RegisterValue struct {
	Value json.RawMessage `json:"value"`
	Meta  HLC             `json:"meta"`
}

type ORSetValue struct {
	Adds    map[string]HLC `json:"adds"`
	Removes map[string]HLC `json:"removes"`
}

type SequenceNode struct {
	ID      string `json:"id"`
	AfterID string `json:"after_id"`
	Value   string `json:"value"`
	Deleted bool   `json:"deleted"`
	Meta    HLC    `json:"meta"`
}

type SequenceValue struct {
	Nodes map[string]SequenceNode `json:"nodes"`
}

type EntityState struct {
	Registers map[string]RegisterValue `json:"registers"`
	Sets      map[string]ORSetValue    `json:"sets"`
	Sequences map[string]SequenceValue `json:"sequences"`
	Tombstone bool                     `json:"tombstone"`
	Meta      HLC                      `json:"meta"`
}

type CRDTState struct {
	SchemaVersion int                    `json:"schema_version"`
	Entities      map[string]EntityState `json:"entities"`
	AppliedOps    map[string]struct{}    `json:"applied_ops"`
	LastApplied   HLC                    `json:"last_applied"`
	LastSyncAt    time.Time              `json:"last_sync_at"`
	PendingOps    int                    `json:"pending_ops"`
	AppliedCount  int64                  `json:"applied_count"`
}

type RegisterPayload struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

type SetPayload struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type InsertSequencePayload struct {
	Field   string `json:"field"`
	LineID  string `json:"line_id"`
	AfterID string `json:"after_id"`
	Value   string `json:"value"`
}

type DeleteSequencePayload struct {
	Field  string `json:"field"`
	LineID string `json:"line_id"`
}

func NewCRDTState() CRDTState {
	return CRDTState{
		SchemaVersion: SchemaVersionV2,
		Entities:      map[string]EntityState{},
		AppliedOps:    map[string]struct{}{},
	}
}

func (s *CRDTState) Clone() CRDTState {
	payload, _ := json.Marshal(s)
	cloned := CRDTState{}
	_ = json.Unmarshal(payload, &cloned)
	if cloned.Entities == nil {
		cloned.Entities = map[string]EntityState{}
	}
	if cloned.AppliedOps == nil {
		cloned.AppliedOps = map[string]struct{}{}
	}
	if cloned.SchemaVersion == 0 {
		cloned.SchemaVersion = SchemaVersionV2
	}
	return cloned
}

func (s *CRDTState) EnsureEntity(entityKey string) EntityState {
	entity, ok := s.Entities[entityKey]
	if !ok {
		entity = EntityState{
			Registers: map[string]RegisterValue{},
			Sets:      map[string]ORSetValue{},
			Sequences: map[string]SequenceValue{},
		}
	}
	if entity.Registers == nil {
		entity.Registers = map[string]RegisterValue{}
	}
	if entity.Sets == nil {
		entity.Sets = map[string]ORSetValue{}
	}
	if entity.Sequences == nil {
		entity.Sequences = map[string]SequenceValue{}
	}
	return entity
}

func (s *CRDTState) Apply(op OpEnvelope) error {
	if err := op.Validate(); err != nil {
		return err
	}
	if _, ok := s.AppliedOps[op.OpID]; ok {
		return nil
	}
	entity := s.EnsureEntity(op.EntityKey)
	meta := ParseHLC(op.HLCTimestamp)

	switch op.OpKind {
	case OpKindPutRegister:
		payload := RegisterPayload{}
		if err := json.Unmarshal(op.Payload, &payload); err != nil {
			return fmt.Errorf("decode put_register payload: %w", err)
		}
		if current, ok := entity.Registers[payload.Field]; !ok || CompareHLC(current.Meta, meta) <= 0 {
			entity.Registers[payload.Field] = RegisterValue{Value: payload.Value, Meta: meta}
		}
	case OpKindAddSet:
		payload := SetPayload{}
		if err := json.Unmarshal(op.Payload, &payload); err != nil {
			return fmt.Errorf("decode add_set payload: %w", err)
		}
		set := entity.Sets[payload.Field]
		if set.Adds == nil {
			set.Adds = map[string]HLC{}
		}
		if set.Removes == nil {
			set.Removes = map[string]HLC{}
		}
		if current, ok := set.Adds[payload.Value]; !ok || CompareHLC(current, meta) <= 0 {
			set.Adds[payload.Value] = meta
		}
		entity.Sets[payload.Field] = set
	case OpKindRemoveSet:
		payload := SetPayload{}
		if err := json.Unmarshal(op.Payload, &payload); err != nil {
			return fmt.Errorf("decode remove_set payload: %w", err)
		}
		set := entity.Sets[payload.Field]
		if set.Adds == nil {
			set.Adds = map[string]HLC{}
		}
		if set.Removes == nil {
			set.Removes = map[string]HLC{}
		}
		if current, ok := set.Removes[payload.Value]; !ok || CompareHLC(current, meta) <= 0 {
			set.Removes[payload.Value] = meta
		}
		entity.Sets[payload.Field] = set
	case OpKindInsertSeq:
		payload := InsertSequencePayload{}
		if err := json.Unmarshal(op.Payload, &payload); err != nil {
			return fmt.Errorf("decode insert_seq payload: %w", err)
		}
		seq := entity.Sequences[payload.Field]
		if seq.Nodes == nil {
			seq.Nodes = map[string]SequenceNode{}
		}
		node := SequenceNode{ID: payload.LineID, AfterID: payload.AfterID, Value: payload.Value, Deleted: false, Meta: meta}
		if current, ok := seq.Nodes[payload.LineID]; !ok || CompareHLC(current.Meta, meta) <= 0 {
			seq.Nodes[payload.LineID] = node
		}
		entity.Sequences[payload.Field] = seq
	case OpKindDeleteSeq:
		payload := DeleteSequencePayload{}
		if err := json.Unmarshal(op.Payload, &payload); err != nil {
			return fmt.Errorf("decode delete_seq payload: %w", err)
		}
		seq := entity.Sequences[payload.Field]
		if seq.Nodes == nil {
			seq.Nodes = map[string]SequenceNode{}
		}
		node := seq.Nodes[payload.LineID]
		if CompareHLC(node.Meta, meta) <= 0 {
			node.ID = payload.LineID
			node.Deleted = true
			node.Meta = meta
			seq.Nodes[payload.LineID] = node
		}
		entity.Sequences[payload.Field] = seq
	case OpKindTombstone:
		if !entity.Tombstone || CompareHLC(entity.Meta, meta) <= 0 {
			entity = EntityState{
				Registers: map[string]RegisterValue{},
				Sets:      map[string]ORSetValue{},
				Sequences: map[string]SequenceValue{},
				Tombstone: true,
				Meta:      meta,
			}
		}
	case OpKindReconcileHint:
		// no-op, used for anti-entropy nudges
	default:
		return fmt.Errorf("%w: %s", ErrUnknownOpKind, op.OpKind)
	}

	if CompareHLC(s.LastApplied, meta) < 0 {
		s.LastApplied = meta
	}
	s.Entities[op.EntityKey] = entity
	s.AppliedOps[op.OpID] = struct{}{}
	s.AppliedCount++
	s.LastSyncAt = time.Now().UTC()
	if s.PendingOps > 0 {
		s.PendingOps--
	}
	if len(s.AppliedOps) > 50000 {
		s.compactAppliedOps()
	}
	return nil
}

func (s *CRDTState) compactAppliedOps() {
	keys := make([]string, 0, len(s.AppliedOps))
	for k := range s.AppliedOps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	keep := 30000
	if len(keys) < keep {
		return
	}
	trimmed := map[string]struct{}{}
	for _, key := range keys[len(keys)-keep:] {
		trimmed[key] = struct{}{}
	}
	s.AppliedOps = trimmed
}

func (e EntityState) RenderSet(field string) []string {
	set := e.Sets[field]
	if len(set.Adds) == 0 {
		return nil
	}
	out := make([]string, 0, len(set.Adds))
	for value, addMeta := range set.Adds {
		removeMeta, removed := set.Removes[value]
		if removed && CompareHLC(addMeta, removeMeta) <= 0 {
			continue
		}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (e EntityState) RenderSequence(field string) []string {
	seq := e.Sequences[field]
	if len(seq.Nodes) == 0 {
		return nil
	}
	children := map[string][]SequenceNode{}
	for _, node := range seq.Nodes {
		children[node.AfterID] = append(children[node.AfterID], node)
	}
	for key := range children {
		sort.Slice(children[key], func(i, j int) bool {
			cmp := CompareHLC(children[key][i].Meta, children[key][j].Meta)
			if cmp == 0 {
				return children[key][i].ID < children[key][j].ID
			}
			return cmp < 0
		})
	}
	lines := make([]string, 0, len(seq.Nodes))
	var walk func(parent string)
	walk = func(parent string) {
		for _, node := range children[parent] {
			if !node.Deleted {
				lines = append(lines, node.Value)
			}
			walk(node.ID)
		}
	}
	walk("")
	return lines
}

func (o OpEnvelope) Signed(key []byte) OpEnvelope {
	clone := o
	clone.AuthTag = ""
	payload, _ := json.Marshal(clone)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	o.AuthTag = hex.EncodeToString(mac.Sum(nil))
	return o
}

func (o OpEnvelope) Verify(key []byte) bool {
	if o.AuthTag == "" {
		return false
	}
	expected := o.Signed(key).AuthTag
	given, err := hex.DecodeString(o.AuthTag)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	return hmac.Equal(given, want)
}

func RandomWorkspaceKey(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if len(decoded) < 32 {
		return nil, fmt.Errorf("workspace key must be at least 32 bytes")
	}
	return decoded, nil
}
