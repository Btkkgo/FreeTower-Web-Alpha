package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func gatheringFixture() (*RuntimeRoom, *Client) {
	c := &Client{user: User{ID: "u"}, player: Player{ID: "u", World: "freetower", Position: Vec2{-13.910968505921797, -17.450264344859693}}, send: make(chan []byte, 100)}
	rr := &RuntimeRoom{record: RoomRecord{Members: map[string]bool{"u": true}}, clients: map[*Client]bool{c: true}}
	rr.ensureGathering()
	c.player.Hands = defaultHands()
	return rr, c
}
func TestGatheringAuthority(t *testing.T) {
	rr, c := gatheringFixture()
	at := int64(10000)
	if err := rr.startGather(c, "tree-1", "basic-axe", "a", at); err != nil {
		t.Fatal(err)
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", at+279, func(RoomRecord) error { return nil }); err == nil {
		t.Fatal("early hit accepted")
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", at+280, func(RoomRecord) error { return errors.New("disk") }); err == nil {
		t.Fatal("save failure accepted")
	}
	if rr.record.State.FreeTower.Nodes["tree-1"].Durability != 3 {
		t.Fatal("failed save mutated durability")
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", at+280, func(RoomRecord) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if rr.record.State.FreeTower.Nodes["tree-1"].Durability != 2 {
		t.Fatal("hit did not damage")
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", at+281, func(RoomRecord) error { return nil }); err == nil {
		t.Fatal("replay accepted")
	}
	if err := rr.startGather(c, "tree-1", "basic-axe", "b", at+300); err == nil {
		t.Fatal("cooldown accepted")
	}
}

func TestGatheringRejections(t *testing.T) {
	tests := []struct {
		name, resource, tool string
		alter                func(*RuntimeRoom, *Client)
	}{
		{"axe-rock", "rock-1", "basic-axe", func(rr *RuntimeRoom, c *Client) {
			c.player.Position = rr.record.State.FreeTower.Nodes["rock-1"].Position
		}},
		{"pickaxe-tree", "tree-1", "basic-pickaxe", func(rr *RuntimeRoom, c *Client) { _ = rr.setHands(c, HandState{"basic-pickaxe", "basic-axe", "left"}) }},
		{"range", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) { c.player.Position = Vec2{0, 0} }},
		{"swimming", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) { c.player.MovementMode = "swimming" }},
		{"ownership", "tree-1", "unknown", func(rr *RuntimeRoom, c *Client) {}},
		{"inactive-hand", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) { _ = rr.setHands(c, HandState{"basic-pickaxe", "basic-axe", "left"}) }},
		{"legacy-world", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) { c.player.World = "" }},
		{"nonmember", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) { delete(rr.record.Members, "u") }},
		{"disconnected", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) { delete(rr.clients, c) }},
		{"unknown-resource", "missing", "basic-axe", func(rr *RuntimeRoom, c *Client) {}},
		{"depleted", "tree-1", "basic-axe", func(rr *RuntimeRoom, c *Client) {
			n := rr.record.State.FreeTower.Nodes["tree-1"]
			n.Depleted = true
			rr.record.State.FreeTower.Nodes["tree-1"] = n
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr, c := gatheringFixture()
			tt.alter(rr, c)
			if err := rr.startGather(c, tt.resource, tt.tool, "a", 10000); err == nil {
				t.Fatal("invalid start accepted")
			}
		})
	}
	for _, kind := range []string{"range", "swimming", "hand", "identity", "expired"} {
		t.Run("hit-"+kind, func(t *testing.T) {
			rr, c := gatheringFixture()
			if err := rr.startGather(c, "tree-1", "basic-axe", "a", 10000); err != nil {
				t.Fatal(err)
			}
			id := "a"
			at := int64(10280)
			switch kind {
			case "range":
				c.player.Position = Vec2{}
			case "swimming":
				c.player.MovementMode = "swimming"
			case "hand":
				rr.gatheringUser(c).Hands.Active = "left"
			case "identity":
				id = "forged"
			case "expired":
				at = 10651
			}
			if err := rr.hitGather(c, "tree-1", "basic-axe", id, at, func(RoomRecord) error { return nil }); err == nil {
				t.Fatal("invalid hit accepted")
			}
			if rr.record.State.FreeTower.Nodes["tree-1"].Durability != 3 {
				t.Fatal("invalid hit changed state")
			}
		})
	}
}
func TestGatheringRewardsAndRespawn(t *testing.T) {
	for _, tt := range []struct {
		rid, tid, item string
		hits, reward   int
		delay          int64
	}{{"tree-1", "basic-axe", "wood", 3, 4, 60000}, {"rock-1", "basic-pickaxe", "stone", 4, 3, 75000}} {
		t.Run(tt.rid, func(t *testing.T) {
			rr, c := gatheringFixture()
			c.player.Position = rr.record.State.FreeTower.Nodes[tt.rid].Position
			if tt.tid == "basic-pickaxe" {
				_ = rr.setHands(c, HandState{"basic-pickaxe", "basic-axe", "left"})
			}
			save := func(RoomRecord) error { return nil }
			var last int64
			for i := 0; i < tt.hits; i++ {
				at := int64(10000 + i*1200)
				id := fmt.Sprint(i)
				if err := rr.startGather(c, tt.rid, tt.tid, id, at); err != nil {
					t.Fatal(err)
				}
				last = at + 280
				if err := rr.hitGather(c, tt.rid, tt.tid, id, last, save); err != nil {
					t.Fatal(err)
				}
			}
			n := rr.record.State.FreeTower.Nodes[tt.rid]
			if !n.Depleted || n.Durability != 0 || n.RespawnAt != last+tt.delay || rr.record.State.FreeTower.Inventory[tt.item] != tt.reward {
				t.Fatalf("depletion/reward: %+v %+v", n, rr.record.State.FreeTower.Inventory)
			}
			if err := rr.startGather(c, tt.rid, tt.tid, "duplicate", last+2000); err == nil {
				t.Fatal("depleted accepted")
			}
			_ = rr.respawnGathering(n.RespawnAt-1, save)
			if !rr.record.State.FreeTower.Nodes[tt.rid].Depleted {
				t.Fatal("early respawn")
			}
			if err := rr.respawnGathering(n.RespawnAt, func(RoomRecord) error { return errors.New("failed") }); err == nil {
				t.Fatal("failed respawn save ignored")
			}
			if !rr.record.State.FreeTower.Nodes[tt.rid].Depleted {
				t.Fatal("failed respawn changed state")
			}
			_ = rr.respawnGathering(n.RespawnAt, save)
			n = rr.record.State.FreeTower.Nodes[tt.rid]
			if n.Depleted || n.Durability != n.MaxDurability || n.RespawnAt != 0 {
				t.Fatal("no respawn")
			}
			if rr.record.State.FreeTower.Inventory[tt.item] != tt.reward {
				t.Fatal("duplicate reward")
			}
		})
	}
}
func TestGatheringHandsAndMultiSocketCooldown(t *testing.T) {
	rr, c := gatheringFixture()
	if rr.setHands(c, HandState{"wood", "basic-axe", "right"}) == nil {
		t.Fatal("unowned wood equipped")
	}
	if rr.setHands(c, HandState{"empty", "basic-axe", "other"}) == nil {
		t.Fatal("invalid active hand")
	}
	rr.record.State.FreeTower.Inventory["wood"] = 4
	if err := rr.setHands(c, HandState{"wood", "basic-axe", "right"}); err != nil {
		t.Fatal(err)
	}
	if rr.record.State.FreeTower.Inventory["wood"] != 4 {
		t.Fatal("equipping consumed sample")
	}
	other := &Client{user: c.user, player: c.player, send: make(chan []byte, 100)}
	rr.clients[other] = true
	if err := rr.startGather(c, "tree-1", "basic-axe", "a", 10000); err != nil {
		t.Fatal(err)
	}
	if other.player.GatherAction == nil || other.player.Hands.Left != "wood" {
		t.Fatal("cross-socket presence not synchronized")
	}
	if rr.startGather(other, "tree-1", "basic-axe", "b", 10001) == nil {
		t.Fatal("overlapping cross socket action")
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", 10280, func(RoomRecord) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if rr.startGather(other, "tree-1", "basic-axe", "b", 10700) == nil {
		t.Fatal("cross socket cooldown bypass")
	}
	rr.cancelGather(c)
	if other.player.GatherAction != nil {
		t.Fatal("cancel did not synchronize")
	}
	if rr.startGather(other, "tree-1", "basic-axe", "a", 12000) == nil {
		t.Fatal("successful ID reused")
	}
}
func TestGatheringSimultaneousLastHits(t *testing.T) {
	rr, c := gatheringFixture()
	other := &Client{user: User{ID: "v"}, player: Player{ID: "v", World: "freetower", Position: c.player.Position}, send: make(chan []byte, 100)}
	rr.clients[other] = true
	rr.record.Members["v"] = true
	n := rr.record.State.FreeTower.Nodes["tree-1"]
	n.Durability = 1
	rr.record.State.FreeTower.Nodes["tree-1"] = n
	for _, client := range []*Client{c, other} {
		if err := rr.startGather(client, "tree-1", "basic-axe", client.user.ID, 10000); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	saves := 0
	for _, client := range []*Client{c, other} {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()
			rr.mu.Lock()
			defer rr.mu.Unlock()
			results <- rr.hitGather(client, "tree-1", "basic-axe", client.user.ID, 10280, func(RoomRecord) error { saves++; return nil })
		}(client)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 || saves != 1 || rr.record.State.FreeTower.Inventory["wood"] != 4 {
		t.Fatalf("success=%d saves=%d reward=%d", success, saves, rr.record.State.FreeTower.Inventory["wood"])
	}
}

func testGatheringRestart(t *testing.T, databaseURL, path string) {
	t.Helper()
	s, err := openStore(databaseURL, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	rr, c := gatheringFixture()
	rr.record.Room = Room{ID: newID(), Code: "GATHER" + newID(), OwnerID: c.user.ID}
	save := func(record RoomRecord) error { return s.saveRoom(context.Background(), record) }
	at := nowMS()
	for i := 0; i < 3; i++ {
		started := at + int64(i*1200)
		id := fmt.Sprint(i)
		if err := rr.startGather(c, "tree-1", "basic-axe", id, started); err != nil {
			t.Fatal(err)
		}
		if err := rr.hitGather(c, "tree-1", "basic-axe", id, started+280, save); err != nil {
			t.Fatal(err)
		}
	}
	before := cloneState(rr.record.State)
	fresh, err := openStore(databaseURL, path)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.close()
	app, err := newApp(fresh, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.shutdown()
	restored := app.rooms[rr.record.Room.ID]
	restored.mu.Lock()
	defer restored.mu.Unlock()
	n := restored.record.State.FreeTower.Nodes["tree-1"]
	if !n.Depleted || n.RespawnAt != before.FreeTower.Nodes["tree-1"].RespawnAt || restored.record.State.FreeTower.Inventory["wood"] != 4 {
		t.Fatalf("restart lost depletion: %+v", n)
	}
	if len(restored.gatherUsers) != 0 {
		t.Fatal("transient action restored")
	}
	if err := restored.respawnGathering(n.RespawnAt, func(record RoomRecord) error { return fresh.saveRoom(context.Background(), record) }); err != nil {
		t.Fatal(err)
	}
	records, err := fresh.loadRooms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Room.ID == rr.record.Room.ID {
			n = record.State.FreeTower.Nodes["tree-1"]
			if n.Depleted || n.Durability != 3 || record.State.FreeTower.Inventory["wood"] != 4 {
				t.Fatal("respawn not durable")
			}
		}
	}
}
func TestGatheringFileRestart(t *testing.T) {
	testGatheringRestart(t, "", filepath.Join(t.TempDir(), "farm.json"))
}
func TestGatheringPostgresRestart(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "ft_gather_test_" + newID()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	testGatheringRestart(t, parsed.String(), "")
}
func TestGatheringLegacySaveOptional(t *testing.T) {
	state := establishedFarm(nowMS())
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("freeTower")) {
		t.Fatal("legacy state unexpectedly gained FreeTower")
	}
	var restored FarmState
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, restored) {
		t.Fatal("legacy roundtrip changed")
	}
}
func TestGatheringHitBroadcastOnlyAfterSave(t *testing.T) {
	rr, c := gatheringFixture()
	_ = rr.startGather(c, "tree-1", "basic-axe", "a", 10000)
	for len(c.send) > 0 {
		<-c.send
	}
	save := func(record RoomRecord) error {
		if len(c.send) != 0 {
			t.Fatal("broadcast before save")
		}
		if record.State.FreeTower.Nodes["tree-1"].Durability != 2 {
			t.Fatal("saved record missing damage")
		}
		return errors.New("fail")
	}
	if rr.hitGather(c, "tree-1", "basic-axe", "a", 10280, save) == nil {
		t.Fatal("save failure ignored")
	}
	if len(c.send) != 0 {
		t.Fatal("failed save broadcast success")
	}
	if rr.gatheringUser(c).Consumed || rr.gatheringUser(c).CooldownUntil != 0 {
		t.Fatal("failed save mutated action/cooldown")
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", 10280, func(RoomRecord) error {
		if len(c.send) != 0 {
			t.Fatal("broadcast before successful save")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(c.send) != 2 {
		t.Fatalf("expected state then hit, got %d", len(c.send))
	}
	var state, event map[string]any
	_ = json.Unmarshal(<-c.send, &state)
	_ = json.Unmarshal(<-c.send, &event)
	if state["type"] != "state" || event["type"] != "resource-hit" {
		t.Fatal("incorrect success wire order")
	}
}

func TestGatheringWebsocketProtocol(t *testing.T) {
	h := newHarness(t)
	a, b := h.client(), h.client()
	h.signup(a, "GatherA")
	h.signup(b, "GatherB")
	var room detail
	h.request(a, "POST", "/api/rooms", map[string]string{"name": "Gather room"}, 201, &room)
	h.request(b, "POST", "/api/rooms/join", map[string]string{"code": room.Room.Code}, 200, nil)
	dial := func(client *http.Client) *websocket.Conn {
		req, _ := http.NewRequest("GET", h.server.URL, nil)
		headers := http.Header{"Origin": []string{h.server.URL}}
		for _, cookie := range client.Jar.Cookies(req.URL) {
			headers.Add("Cookie", cookie.String())
		}
		ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.server.URL, "http")+"/api/rooms/"+room.Room.ID+"/ws?world=freetower", headers)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ws.Close() })
		return ws
	}
	wa, wb := dial(a), dial(b)
	read := func(ws *websocket.Conn, kind string) map[string]json.RawMessage {
		t.Helper()
		_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			var msg map[string]json.RawMessage
			if err := ws.ReadJSON(&msg); err != nil {
				t.Fatal(err)
			}
			var typ string
			_ = json.Unmarshal(msg["type"], &typ)
			if typ == kind {
				return msg
			}
		}
	}
	_ = read(wa, "state")
	_ = read(wb, "state")
	rr := h.app.rooms[room.Room.ID]
	rr.mu.Lock()
	for c := range rr.clients {
		if c.user.Username == "GatherA" {
			c.player.Position = rr.record.State.FreeTower.Nodes["tree-1"].Position
		}
	}
	rr.mu.Unlock()
	if err := wa.WriteJSON(map[string]any{"type": "hands-set", "hands": HandState{"empty", "basic-axe", "right"}}); err != nil {
		t.Fatal(err)
	}
	for {
		msg := read(wb, "presence")
		var players []Player
		_ = json.Unmarshal(msg["players"], &players)
		found := false
		for _, p := range players {
			if p.Username == "GatherA" && p.Hands != nil && p.Hands.Left == "empty" {
				found = true
			}
		}
		if found {
			break
		}
	}
	if err := wa.WriteJSON(map[string]any{"type": "gather-start", "resourceId": "tree-1", "toolId": "basic-axe", "actionId": "wire"}); err != nil {
		t.Fatal(err)
	}
	var action GatherAction
	for {
		msg := read(wb, "presence")
		var players []Player
		_ = json.Unmarshal(msg["players"], &players)
		for _, p := range players {
			if p.Username == "GatherA" && p.GatherAction != nil {
				action = *p.GatherAction
			}
		}
		if action.ID != "" {
			break
		}
	}
	if action.HitAt-action.StartedAt != 280 || action.EndsAt-action.HitAt != 370 {
		t.Fatalf("server timing: %+v", action)
	}
	time.Sleep(time.Until(time.UnixMilli(action.HitAt + 15)))
	if err := wa.WriteJSON(map[string]any{"type": "resource-hit", "resourceId": "tree-1", "toolId": "basic-axe", "actionId": "wire"}); err != nil {
		t.Fatal(err)
	}
	event := read(wb, "resource-hit")
	var id string
	_ = json.Unmarshal(event["actionId"], &id)
	if id != "wire" {
		t.Fatal("hit action missing")
	}
	rr.mu.Lock()
	durability := rr.record.State.FreeTower.Nodes["tree-1"].Durability
	rr.mu.Unlock()
	if durability != 2 {
		t.Fatal("wire hit not authoritative")
	}
	if err := wa.WriteJSON(map[string]any{"type": "resource-hit", "resourceId": "tree-1", "toolId": "basic-axe", "actionId": "wire"}); err != nil {
		t.Fatal(err)
	}
	errMsg := read(wa, "gather-error")
	var code string
	_ = json.Unmarshal(errMsg["code"], &code)
	if code != "action" {
		t.Fatalf("replay error %s", code)
	}
}
func TestGatheringRealStoreFailedWrite(t *testing.T) {
	rr, c := gatheringFixture()
	store, err := openStore("", filepath.Join(t.TempDir(), "farm.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	store.path = t.TempDir()
	_ = rr.startGather(c, "tree-1", "basic-axe", "a", 10000)
	if err := rr.hitGather(c, "tree-1", "basic-axe", "a", 10280, func(r RoomRecord) error { return store.saveRoom(context.Background(), r) }); err == nil {
		t.Fatal("real failed file write accepted")
	}
	if rr.record.State.FreeTower.Nodes["tree-1"].Durability != 3 {
		t.Fatal("real failed store mutated room")
	}
	if len(store.data.Rooms) != 0 {
		t.Fatal("real failed store did not roll back")
	}
}

func TestGatheringMutationVersionsAndCooldownBoundary(t *testing.T) {
	rr, c := gatheringFixture()
	rr.record.State.Version = 7
	save := func(RoomRecord) error { return nil }
	failed := func(RoomRecord) error { return errors.New("fail") }
	if err := rr.startGather(c, "tree-1", "basic-axe", "v1", 10000); err != nil {
		t.Fatal(err)
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "v1", 10280, failed); err == nil {
		t.Fatal("write accepted")
	}
	if rr.record.State.Version != 7 || rr.record.State.FreeTower.Version != 1 || rr.record.State.UpdatedAt != 0 {
		t.Fatal("failed hit incremented version/time")
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "v1", 10280, save); err != nil {
		t.Fatal(err)
	}
	if rr.record.State.Version != 8 || rr.record.State.FreeTower.Version != 2 || rr.record.State.UpdatedAt != 10280 {
		t.Fatal("hit missing monotonic versions/time")
	}
	if err := rr.startGather(c, "tree-1", "basic-axe", "v2", 10799); err == nil {
		t.Fatal("start before exact 800ms boundary accepted")
	}
	if err := rr.startGather(c, "tree-1", "basic-axe", "v2", 10800); err != nil {
		t.Fatalf("800ms start boundary rejected: %v", err)
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "v2", 11080, save); err != nil {
		t.Fatal(err)
	}
	if err := rr.startGather(c, "tree-1", "basic-axe", "v3", 11600); err != nil {
		t.Fatal(err)
	}
	if err := rr.hitGather(c, "tree-1", "basic-axe", "v3", 11880, save); err != nil {
		t.Fatal(err)
	}
	deadline := rr.record.State.FreeTower.Nodes["tree-1"].RespawnAt
	before := cloneState(rr.record.State)
	_ = rr.respawnGathering(deadline-1, save)
	if rr.record.State.Version != before.Version || rr.record.State.FreeTower.Version != before.FreeTower.Version {
		t.Fatal("unchanged respawn incremented version")
	}
	if rr.respawnGathering(deadline, failed) == nil {
		t.Fatal("failed respawn accepted")
	}
	if rr.record.State.Version != before.Version || rr.record.State.FreeTower.Version != before.FreeTower.Version || rr.record.State.UpdatedAt != before.UpdatedAt {
		t.Fatal("failed respawn incremented version/time")
	}
	if err := rr.respawnGathering(deadline, save); err != nil {
		t.Fatal(err)
	}
	if rr.record.State.Version != before.Version+1 || rr.record.State.FreeTower.Version != before.FreeTower.Version+1 || rr.record.State.UpdatedAt != deadline {
		t.Fatal("successful respawn missing monotonic versions/time")
	}
}
