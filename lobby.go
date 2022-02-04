package lobby

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultTimeout = time.Minute
)

// GameInfo is a full information for a registered Nox game, as returned by the Lobby.
// It extends Game with additional information.
type GameInfo struct {
	Game
	SeenAt time.Time `json:"seen_at,omitempty"`
}

func (g *GameInfo) Clone() *GameInfo {
	if g == nil {
		return nil
	}
	g2 := *g
	g2.Game = *g.Game.Clone()
	return &g2
}

// Registerer is an interface for registering Nox games on lobby server.
type Registerer interface {
	// RegisterGame registers new game or updates the registration for existing game.
	// The client must call this method periodically to not let the game registration to expire.
	// Using a duration smaller than DefaultTimeout is advised.
	RegisterGame(ctx context.Context, s *Game) error
}

// Lister is an interface for listing Nox games registered on lobby server.
type Lister interface {
	// ListGames returns a sorted list of games registered on this lobby.
	ListGames(ctx context.Context) ([]GameInfo, error)
}

// Lobby is a Nox game lobby for listing and registering games.
type Lobby interface {
	Registerer
	Lister
}

// KeepRegistered keeps registering server so that it doesn't expire.
//
// The update channel sets a pace for updates. If it's set to nil, a default duration will be used.
//
// Once the channel triggers, GameHost.GameInfo is called to acquire fresh game info.
//
// The function returns when context is canceled, if an error is returned from GameHost.GameInfo,
// or if lobby becomes unavailable.
func KeepRegistered(ctx context.Context, l Registerer, update <-chan time.Time, h GameHost) error {
	if update == nil {
		ticker := time.NewTicker(DefaultTimeout / 3)
		defer ticker.Stop()
		update = ticker.C
	}
	failure := 0
	for {
		sctx, cancel := context.WithTimeout(ctx, DefaultTimeout/3)
		info, err := h.GameInfo(sctx)
		cancel()
		if err != nil {
			return err
		}
		err = l.RegisterGame(ctx, info)
		if err != nil {
			failure++
			if failure > 3 {
				return err
			}
		} else {
			failure = 0
		}
		select {
		case <-ctx.Done():
			return nil
		case <-update:
		}
	}
}

func (g Game) gameKey() gameKey {
	return gameKey{
		Addr: g.Address,
		Port: g.Port,
	}
}

type gameKey struct {
	Addr string
	Port int
}

var _ Lobby = (*Service)(nil)

// NewLobby creates a new in-memory Lobby.
func NewLobby() *Service {
	return &Service{
		byAddr:  make(map[gameKey]*GameInfo),
		timeout: DefaultTimeout,
	}
}

// Service is an in-memory implementation of a Lobby.
type Service struct {
	gc      int32 // atomic
	mu      sync.RWMutex
	byAddr  map[gameKey]*GameInfo
	timeout time.Duration
}

// SetTimeout sets an expiration time for game registrations.
func (l *Service) SetTimeout(dt time.Duration) {
	l.mu.Lock()
	l.timeout = dt
	l.mu.Unlock()
}

// RegisterGame implements Lobby.
func (l *Service) RegisterGame(ctx context.Context, s *Game) error {
	if s.Players.Cur < 0 {
		return errors.New("players number should be positive")
	}
	if s.Players.Max <= 0 {
		return errors.New("max players number should be set")
	}
	if s.Address == "" {
		return errors.New("address must be set")
	}
	if s.Vers == "" {
		return errors.New("version should be set")
	}
	if s.Map == "" {
		return errors.New("map should be set")
	}
	if s.Mode == "" {
		return errors.New("mode should be set")
	}
	if s.Name == "" || s.Name != strings.TrimSpace(s.Name) {
		return errors.New("invalid server name")
	}
	if s.Port <= 0 {
		s.Port = DefaultGamePort
	}
	now := time.Now().UTC()
	info := &GameInfo{Game: *s, SeenAt: now}
	key := s.gameKey()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.byAddr[key] = info
	l.maybeGC(now)
	return nil
}

func (l *Service) maybeGC(now time.Time) {
	if !atomic.CompareAndSwapInt32(&l.gc, 1, 0) {
		return
	}
	for k, v := range l.byAddr {
		if !l.isValid(v, now) {
			delete(l.byAddr, k)
		}
	}
}

func (l *Service) triggerGC() {
	atomic.StoreInt32(&l.gc, 1)
}

func (l *Service) isValid(v *GameInfo, now time.Time) bool {
	return v.SeenAt.Add(l.timeout).After(now)
}

func (l *Service) listGames() []GameInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()
	now := time.Now()
	out := make([]GameInfo, 0, len(l.byAddr))
	for _, v := range l.byAddr {
		if l.isValid(v, now) {
			out = append(out, *v.Clone())
		} else {
			l.triggerGC()
		}
	}
	return out
}

// ListGames implements Lobby.
func (l *Service) ListGames(ctx context.Context) ([]GameInfo, error) {
	list := l.listGames()
	sortGameInfos(list)
	return list, nil
}

func sortGameInfos(list []GameInfo) {
	sort.Slice(list, func(i, j int) bool {
		a, b := &list[i], &list[j]
		if a.Address != b.Address {
			return a.Address < b.Address
		}
		return a.Port < b.Port
	})
}