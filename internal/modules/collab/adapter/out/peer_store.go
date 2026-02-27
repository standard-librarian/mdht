package out

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type FilePeerStore struct {
	path string
}

func NewFilePeerStore(vaultPath string) collabout.PeerStore {
	return &FilePeerStore{path: filepath.Join(vaultPath, ".mdht", "collab", "peers.json")}
}

func (s *FilePeerStore) Add(ctx context.Context, addr, label string) (domain.Peer, error) {
	_ = ctx
	if strings.TrimSpace(addr) == "" {
		return domain.Peer{}, domain.ErrInvalidPeerAddress
	}
	if _, err := multiaddr.NewMultiaddr(addr); err != nil {
		return domain.Peer{}, fmt.Errorf("%w: %v", domain.ErrInvalidPeerAddress, err)
	}
	peerID := parsePeerID(addr)
	if peerID == "" {
		return domain.Peer{}, domain.ErrInvalidPeerAddress
	}
	peers, err := s.List(ctx)
	if err != nil {
		return domain.Peer{}, err
	}
	for i, item := range peers {
		if item.PeerID != peerID {
			continue
		}
		peers[i].Address = addr
		peers[i].Label = label
		peers[i].LastSeen = time.Now().UTC()
		if peers[i].Reachability == "" {
			peers[i].Reachability = domain.ReachabilityUnknown
		}
		if peers[i].TraversalMode == "" {
			peers[i].TraversalMode = domain.TraversalDirect
		}
		if peers[i].LastDialResult == "" {
			peers[i].LastDialResult = domain.DialResultUnknown
		}
		if err := s.write(peers); err != nil {
			return domain.Peer{}, err
		}
		return peers[i], nil
	}
	now := time.Now().UTC()
	peer := domain.Peer{
		PeerID:         peerID,
		Address:        addr,
		Label:          label,
		State:          domain.PeerStatePending,
		FirstSeen:      now,
		LastSeen:       now,
		AddedAt:        now,
		Reachability:   domain.ReachabilityUnknown,
		LastDialResult: domain.DialResultUnknown,
		TraversalMode:  domain.TraversalDirect,
	}
	peers = append(peers, peer)
	if err := s.write(peers); err != nil {
		return domain.Peer{}, err
	}
	return peer, nil
}

func (s *FilePeerStore) Approve(ctx context.Context, peerID string) (domain.Peer, error) {
	return s.updateState(ctx, peerID, domain.PeerStateApproved)
}

func (s *FilePeerStore) Revoke(ctx context.Context, peerID string) (domain.Peer, error) {
	return s.updateState(ctx, peerID, domain.PeerStateRevoked)
}

func (s *FilePeerStore) Remove(ctx context.Context, peerID string) error {
	_ = ctx
	peers, err := s.List(ctx)
	if err != nil {
		return err
	}
	filtered := make([]domain.Peer, 0, len(peers))
	removed := false
	for _, peer := range peers {
		if peer.PeerID == peerID {
			removed = true
			continue
		}
		filtered = append(filtered, peer)
	}
	if !removed {
		return domain.ErrPeerNotFound
	}
	return s.write(filtered)
}

func (s *FilePeerStore) Update(ctx context.Context, peer domain.Peer) (domain.Peer, error) {
	_ = ctx
	peers, err := s.List(ctx)
	if err != nil {
		return domain.Peer{}, err
	}
	for i := range peers {
		if peers[i].PeerID != peer.PeerID {
			continue
		}
		if peer.Address != "" {
			peers[i].Address = peer.Address
		}
		if peer.Label != "" {
			peers[i].Label = peer.Label
		}
		if peer.State != "" {
			peers[i].State = peer.State
		}
		if !peer.FirstSeen.IsZero() {
			peers[i].FirstSeen = peer.FirstSeen
		}
		if !peer.LastSeen.IsZero() {
			peers[i].LastSeen = peer.LastSeen
		}
		if !peer.LastDialAt.IsZero() {
			peers[i].LastDialAt = peer.LastDialAt
		}
		if peer.LastDialResult != "" {
			peers[i].LastDialResult = peer.LastDialResult
		}
		if peer.RTTMS > 0 {
			peers[i].RTTMS = peer.RTTMS
		}
		if peer.TraversalMode != "" {
			peers[i].TraversalMode = peer.TraversalMode
		}
		if peer.Reachability != "" {
			peers[i].Reachability = peer.Reachability
		}
		if peer.LastError != "" {
			peers[i].LastError = peer.LastError
		}
		if err := s.write(peers); err != nil {
			return domain.Peer{}, err
		}
		return peers[i], nil
	}
	return domain.Peer{}, domain.ErrPeerNotFound
}

func (s *FilePeerStore) List(_ context.Context) ([]domain.Peer, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.Peer{}, nil
		}
		return nil, fmt.Errorf("read peers: %w", err)
	}
	peers := []domain.Peer{}
	if len(raw) == 0 {
		return peers, nil
	}
	if err := json.Unmarshal(raw, &peers); err != nil {
		return nil, fmt.Errorf("decode peers: %w", err)
	}
	for i := range peers {
		if peers[i].State == "" {
			peers[i].State = domain.PeerStatePending
		}
		if peers[i].Reachability == "" {
			peers[i].Reachability = domain.ReachabilityUnknown
		}
		if peers[i].TraversalMode == "" {
			peers[i].TraversalMode = domain.TraversalDirect
		}
		if peers[i].LastDialResult == "" {
			peers[i].LastDialResult = domain.DialResultUnknown
		}
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].PeerID < peers[j].PeerID
	})
	return peers, nil
}

func (s *FilePeerStore) updateState(ctx context.Context, peerID string, state domain.PeerState) (domain.Peer, error) {
	peers, err := s.List(ctx)
	if err != nil {
		return domain.Peer{}, err
	}
	for i := range peers {
		if peers[i].PeerID != peerID {
			continue
		}
		peers[i].State = state
		peers[i].LastSeen = time.Now().UTC()
		if err := s.write(peers); err != nil {
			return domain.Peer{}, err
		}
		return peers[i], nil
	}
	return domain.Peer{}, domain.ErrPeerNotFound
}

func (s *FilePeerStore) write(peers []domain.Peer) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create peers dir: %w", err)
	}
	payload, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return fmt.Errorf("encode peers: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return fmt.Errorf("write peers: %w", err)
	}
	return nil
}

func parsePeerID(addr string) string {
	parts := strings.Split(strings.TrimSpace(addr), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "p2p" {
			return parts[i+1]
		}
	}
	return ""
}
