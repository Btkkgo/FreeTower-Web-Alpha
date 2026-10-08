package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

type Client struct {
	sessionHash string
	conn        *websocket.Conn
	user        User
	player      Player
	send        chan []byte
	done        chan struct{}
	once        sync.Once
}

func (c *Client) revokeIdentity() {
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Identity authentication required"), time.Now().Add(time.Second))
	c.close()
}
func (c *Client) close() { c.once.Do(func() { close(c.done); _ = c.conn.Close() }) }
func (rr *RuntimeRoom) broadcast(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	for c := range rr.clients {
		select {
		case c.send <- b:
		default:
			c.close()
		}
	}
}
func (rr *RuntimeRoom) players() []Player {
	out := []Player{}
	seen := map[string]bool{}
	for c := range rr.clients {
		if !seen[c.user.ID] {
			out = append(out, c.player)
			seen[c.user.ID] = true
		}
	}
	return out
}

// Membership and chat stay shared; coordinate spaces are separate presentations.
func (rr *RuntimeRoom) playersInWorld(world string) []Player {
	out := []Player{}
	seen := map[string]bool{}
	for c := range rr.clients {
		if c.player.World == world && !seen[c.user.ID] {
			out = append(out, c.player)
			seen[c.user.ID] = true
		}
	}
	return out
}
func (rr *RuntimeRoom) broadcastPresence() {
	payloads := map[string][]byte{}
	for c := range rr.clients {
		b, ok := payloads[c.player.World]
		if !ok {
			b, _ = json.Marshal(map[string]any{"type": "presence", "players": rr.playersInWorld(c.player.World)})
			payloads[c.player.World] = b
		}
		select {
		case c.send <- b:
		default:
			c.close()
		}
	}
}

var upgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 4096, CheckOrigin: sameOrigin}

func (a *App) websocket(w http.ResponseWriter, r *http.Request) {
	// Serialize registration against logout/re-auth so an upgrade cannot escape revocation.
	a.identity.mu.Lock()
	if a.closing || a.runtime.Public() && !a.store.worldWriterActive() {
		a.identity.mu.Unlock()
		jsonError(w, 503, "FreeTower is restarting")
		return
	}
	a.websocketDone.Add(1)
	defer a.websocketDone.Done()
	locked := true
	defer func() {
		if locked {
			a.identity.mu.Unlock()
		}
	}()
	if requestWalletHash(r) != "" && !a.identity.originAllowed(r) {
		identityError(w, 403, "origin_mismatch", "Invalid world origin")
		return
	}
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rr := a.roomFor(w, r, u)
	if rr == nil {
		return
	}
	rr.mu.Lock()
	if rr.deleted {
		rr.mu.Unlock()
		jsonError(w, 404, "这座牧场已经被主人删除了")
		return
	}
	if len(rr.clients) >= 24 {
		rr.mu.Unlock()
		jsonError(w, 429, "农场当前连接较多，稍后再来吧")
		return
	}
	upgrade := upgrader
	upgrade.CheckOrigin = a.runtime.originAllowed
	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		rr.mu.Unlock()
		return
	}
	colors := []string{"#de9479", "#89b7a1", "#aca1cc", "#d4b575", "#87aec8"}
	c := &Client{sessionHash: requestWalletHash(r), conn: conn, user: u, player: Player{CharacterIdentityID: u.CharacterIdentityID, BtcName: u.BtcName, CharacterAssetID: u.CharacterAssetID, IdentityMode: u.IdentityMode, ID: u.ID, Username: u.Username, Avatar: u.Avatar, Position: Vec2{1, 7}, Color: colors[len(rr.clients)%len(colors)]}, send: make(chan []byte, 64), done: make(chan struct{})}
	if r.URL.Query().Get("world") == "freetower" {
		c.player.World = "freetower"
		rr.ensureGathering()
		c.player.Hands = defaultHands()
		c.player.Position = freeTowerConfig.Spawn
		c.player.MovementMode, c.player.Elevation = freeTowerLocomotion(c.player.Position)
	}
	rr.tick(nowMS())
	rr.clients[c] = true
	if c.player.World == "freetower" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := rr.joinLife(c, nowMS(), func(record RoomRecord) error { return a.store.saveRoom(ctx, record) }); err != nil {
			delete(rr.clients, c)
			rr.mu.Unlock()
			c.close()
			cancel()
			return
		}
		cancel()
		rr.gatheringUser(c)
		rr.syncGatherUser(c.user.ID)
	}
	rr.broadcastPresence()
	b, _ := json.Marshal(map[string]any{"type": "state", "state": publicState(rr.record.State), "serverTime": nowMS()})
	c.send <- b
	rr.mu.Unlock()
	if c.sessionHash != "" {
		session, _, _, e := a.walletCurrent(r.Context(), c.sessionHash)
		if e != nil {
			c.close()
		} else {
			timer := time.AfterFunc(time.Until(time.UnixMilli(session.ExpiresAt)), c.revokeIdentity)
			defer timer.Stop()
		}
	}
	a.identity.mu.Unlock()
	locked = false
	go c.writePump()
	c.readPump(a, rr)
	rr.mu.Lock()
	rr.tick(nowMS())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = rr.advanceBuildings(nowMS(), false, c, func(record RoomRecord) error { return a.store.saveRoom(ctx, record) })
	_ = rr.advanceLife(nowMS(), false, c, func(record RoomRecord) error { return a.store.saveRoom(ctx, record) })
	cancel()
	delete(rr.clients, c)
	delete(rr.lifeReplaySync, c)
	delete(rr.lifeReplayReply, c)
	rr.broadcastPresence()
	rr.mu.Unlock()
	c.close()
}
func (c *Client) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	defer c.close()
	for {
		select {
		case <-c.done:
			return
		case b, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if c.conn.WriteMessage(websocket.TextMessage, b) != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if c.conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}
}
func (c *Client) readPump(a *App, rr *RuntimeRoom) {
	c.conn.SetReadLimit(2048)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	lastMove := time.Now().Add(-time.Second)
	lastChat := time.Time{}
	lastWhistle := time.Time{}
	for {
		_, b, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		stop := func() bool {
			a.identity.mu.Lock()
			defer a.identity.mu.Unlock()
			select {
			case <-c.done:
				return true
			default:
			}
			if a.runtime.Public() && !a.store.worldWriterActive() {
				return true
			}
			if c.sessionHash != "" {
				_, _, i, e := a.walletCurrent(context.Background(), c.sessionHash)
				if e != nil || i == nil || i.ID != c.user.ID {
					return true
				}
			}
			var msg struct {
				Type       string    `json:"type"`
				Hands      HandState `json:"hands"`
				ResourceID string    `json:"resourceId"`
				ToolID     string    `json:"toolId"`
				ActionID   string    `json:"actionId"`
				Position   Vec2      `json:"position"`
				Facing     float64   `json:"facing"`
				Jump       bool      `json:"jump"`
				Moving     bool      `json:"moving"`
				Running    bool      `json:"running"`
				Text       string    `json:"text"`
			}
			if json.Unmarshal(b, &msg) != nil {
				return false
			}
			switch msg.Type {
			case "life-action":
				var q LifeRequest
				if json.Unmarshal(b, &q) != nil {
					return false
				}
				rr.mu.Lock()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				at := nowMS()
				replay := false
				if rr.record.State.FreeTower != nil {
					_, replay = rr.record.State.FreeTower.LifeReceipts[q.RequestID]
				}
				if _, ok := rr.lifeNoopReceipts[q.RequestID]; ok {
					replay = true
				}
				failure := rr.lifeCommand(c, q, at, func(record RoomRecord) error { return a.store.saveRoom(ctx, record) })
				cancel()
				if replay && failure == nil && !rr.lifeReplayReplyAllowed(c, at) {
					rr.mu.Unlock()
					return false
				}
				payload := map[string]any{"type": "life-result", "requestId": q.RequestID, "message": "Life action completed"}
				if failure != nil {
					code := "invalid"
					if e, ok := failure.(*gatherError); ok {
						code = e.Code
					}
					payload = map[string]any{"type": "life-error", "requestId": q.RequestID, "code": code, "message": failure.Error()}
				}
				raw, _ := json.Marshal(payload)
				select {
				case c.send <- raw:
				default:
					c.close()
				}
				rr.mu.Unlock()
			case "place-building", "begin-building-move", "move-building", "cancel-building-move", "renew-building-move":
				var q BuildingRequest
				if json.Unmarshal(b, &q) != nil {
					return false
				}
				rr.mu.Lock()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				result, failure := rr.buildingCommand(c, msg.Type, q, nowMS(), func(record RoomRecord) error { return a.store.saveRoom(ctx, record) })
				cancel()
				var payload []byte
				if failure != nil {
					code := "invalid"
					if e, ok := failure.(*gatherError); ok {
						code = e.Code
					}
					payload, _ = json.Marshal(map[string]any{"type": "building-error", "requestId": q.RequestID, "code": code, "message": failure.Error()})
				} else {
					payload, _ = json.Marshal(map[string]any{"type": "building-result", "requestId": result.RequestID, "buildingId": result.BuildingID, "moveToken": result.MoveToken})
				}
				select {
				case c.send <- payload:
				default:
					c.close()
				}
				rr.mu.Unlock()
			case "hands-set", "gather-start", "resource-hit", "gather-cancel":
				rr.mu.Lock()
				at := nowMS()
				var gatherFailure error
				switch msg.Type {
				case "hands-set":
					gatherFailure = rr.setHands(c, msg.Hands)
				case "gather-start":
					gatherFailure = rr.startGather(c, msg.ResourceID, msg.ToolID, msg.ActionID, at)
				case "resource-hit":
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					gatherFailure = rr.hitGather(c, msg.ResourceID, msg.ToolID, msg.ActionID, at, func(record RoomRecord) error { return a.store.saveRoom(ctx, record) })
					cancel()
				case "gather-cancel":
					rr.cancelGather(c)
					rr.broadcastPresence()
				}
				if gatherFailure != nil {
					code := "invalid"
					if e, ok := gatherFailure.(*gatherError); ok {
						code = e.Code
					}
					payload, _ := json.Marshal(map[string]any{"type": "gather-error", "code": code, "message": gatherFailure.Error()})
					select {
					case c.send <- payload:
					default:
						c.close()
					}
				}
				rr.mu.Unlock()
			case "move":
				now := time.Now()
				dt := now.Sub(lastMove).Seconds()
				if dt < .04 {
					return false
				}
				if math.IsNaN(msg.Position.X) || math.IsInf(msg.Position.X, 0) || math.IsNaN(msg.Position.Z) || math.IsInf(msg.Position.Z, 0) || math.IsNaN(msg.Facing) || math.IsInf(msg.Facing, 0) {
					return false
				}
				rr.mu.Lock()
				dx := msg.Position.X - c.player.Position.X
				dz := msg.Position.Z - c.player.Position.Z
				distance := math.Hypot(dx, dz)
				speed := 14.0
				if c.player.World == "freetower" {
					speed = freeTowerConfig.Movement.Run
				}
				limit := math.Min(dt, 2)*speed + 1
				if distance > limit {
					msg.Position.X = c.player.Position.X + dx/distance*limit
					msg.Position.Z = c.player.Position.Z + dz/distance*limit
				}
				rr.tick(now.UnixMilli())
				if c.player.World == "freetower" {
					c.player.Position = buildingMovement(rr.record.State.FreeTower, c.player.Position, freeTowerPosition(msg.Position))
					c.player.MovementMode, c.player.Elevation = freeTowerLocomotion(c.player.Position)
					if distance > .01 || c.player.MovementMode == "swimming" {
						rr.cancelGather(c)
					}
					c.player.Moving = msg.Moving
					c.player.Running = msg.Running && c.player.MovementMode == "land"
				} else {
					c.player.Position = allowedPosition(&rr.record.State, msg.Position, c.player.Position)
				}
				c.player.Facing = msg.Facing
				if msg.Jump && c.player.MovementMode != "swimming" && now.UnixMilli()-c.player.JumpAt >= 650 {
					c.player.JumpAt = now.UnixMilli()
				}
				rr.broadcastPresence()
				rr.mu.Unlock()
				lastMove = now
			case "chat":
				text := strings.TrimSpace(msg.Text)
				if utf8.RuneCountInString(text) < 1 || utf8.RuneCountInString(text) > 180 || time.Since(lastChat) < time.Second {
					return false
				}
				lastChat = time.Now()
				rr.mu.Lock()
				rr.broadcast(map[string]any{"type": "chat", "userId": c.user.ID, "username": c.user.Username, "characterIdentityId": c.user.CharacterIdentityID, "btcName": c.user.BtcName, "characterAssetId": c.user.CharacterAssetID, "text": text, "at": nowMS()})
				rr.mu.Unlock()
			case "whistle":
				if time.Since(lastWhistle) < 3*time.Second {
					return false
				}
				lastWhistle = time.Now()
				rr.mu.Lock()
				rr.broadcast(map[string]any{"type": "whistle", "position": c.player.Position, "at": lastWhistle.UnixMilli()})
				rr.mu.Unlock()
			}
			return false
		}()
		if stop {
			return
		}
	}
}
