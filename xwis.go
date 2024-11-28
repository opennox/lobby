package lobby

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/opennox/xwis"
)

// NewXWISWithClient creates a Lister for a Nox XWIS lobby using an existing xwis.Client.
func NewXWISWithClient(c *xwis.Client) Lister {
	return &xwisLister{c: c}
}

func xwisGameMode(v xwis.MapType) GameMode {
	switch v {
	case xwis.MapTypeKOTR:
		return ModeKOTR
	case xwis.MapTypeCTF:
		return ModeCTF
	case xwis.MapTypeFlagBall:
		return ModeFlagBall
	case xwis.MapTypeChat:
		return ModeChat
	case xwis.MapTypeArena:
		return ModeArena
	case xwis.MapTypeElimination:
		return ModeElimination
	case xwis.MapTypeCoop:
		return ModeCoop
	case xwis.MapTypeQuest:
		return ModeQuest
	}
	return ModeCustom
}

func xwisAccess(v xwis.Access) GameAccess {
	switch v {
	case xwis.AccessOpen:
		return AccessOpen
	case xwis.AccessClosed:
		return AccessClosed
	case xwis.AccessPrivate:
		return AccessPassword
	}
	return ""
}

// GameFromXWIS convert xwis.GameInfo to Game type defined by lobby.
func GameFromXWIS(g *xwis.GameInfo) *Game {
	var q *QuestInfo
	if g.MapType == xwis.MapTypeQuest {
		q = &QuestInfo{Stage: g.FragLimit}
	}
	res := Resolution{HighRes: false}
	switch g.Resolution {
	case xwis.Res640x480:
		res.Width, res.Height = 640, 480
	case xwis.Res800x600:
		res.Width, res.Height = 800, 600
	case xwis.Res1024x768:
		res.Width, res.Height = 1024, 768
	}
	return &Game{
		Name:    g.Name,
		Address: g.Addr,
		Port:    DefaultGamePort, // TODO
		Map:     strings.ToLower(g.Map),
		Mode:    xwisGameMode(g.MapType),
		Access:  xwisAccess(g.Access),
		Players: PlayersInfo{
			Cur: g.Players,
			Max: g.MaxPlayers,
		},
		Res:   res,
		Quest: q,
	}
}

type xwisLister struct {
	mu   sync.Mutex
	c    *xwis.Client
	prev map[string][]string
}

func (l *xwisLister) metricsForRooms(list []GameInfo) {
	cntXWISGames.Set(float64(len(list)))
	seen := make(map[string][]string, len(list))
	for _, v := range list {
		labels := serverLabels(sourceXWIS, &v.Game)
		seen[strings.Join(labels, ",")] = labels
		cntGameSeen.WithLabelValues(labels...).Inc()
		cntGamePlayers.WithLabelValues(labels...).Set(float64(v.Players.Cur))
	}
	for k, v := range l.prev {
		if _, ok := seen[k]; !ok {
			cntGamePlayers.WithLabelValues(v...).Set(0)
		}
	}
	l.prev = seen
}

func (l *xwisLister) ListGames(ctx context.Context) ([]GameInfo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	list, err := l.c.ListRooms(ctx)
	if err != nil {
		return nil, err
	}
	cntXWISRooms.Set(float64(len(list)))
	now := time.Now().UTC()
	var out []GameInfo
	for _, r := range list {
		g := r.Game
		if g == nil {
			continue
		}
		v := GameFromXWIS(g)
		out = append(out, GameInfo{Game: *v, SeenAt: now})
	}
	l.metricsForRooms(out)
	log.Printf("xwis: %d rooms, %d games", len(list), len(out))
	return out, nil
}
