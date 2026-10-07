package main

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIdentityStalledProviderDoesNotBlockLogout(t *testing.T) {
	h := newIdentityHarness(t)
	f := h.fixtureProviders()
	h.login()
	h.selectIdentity(200)
	proof := h.proof(h.challenge())
	f.wait = make(chan struct{})
	f.started = make(chan struct{})
	done := make(chan int, 1)
	go func() {
		status, _ := h.request(h.client, "POST", "/api/auth/wallet/verify", proof, h.server.URL)
		done <- status
	}()
	<-f.started
	loggedOut := make(chan int, 1)
	go func() {
		status, _ := h.request(h.client, "POST", "/api/auth/wallet/logout", map[string]string{}, h.server.URL)
		loggedOut <- status
	}()
	select {
	case code := <-loggedOut:
		if code != 200 {
			close(f.wait)
			t.Fatalf("logout %d", code)
		}
	case <-time.After(500 * time.Millisecond):
		close(f.wait)
		t.Fatal("provider stalled authentication mutex")
	}
	close(f.wait)
	if code := <-done; code != 401 {
		t.Fatalf("in-flight login revived after logout: %d", code)
	}
	h.call("GET", "/api/identity/me", nil, 401, nil)
}
func TestIdentityOwnershipPersistenceFailureRevokesAuthority(t *testing.T) {
	for _, kind := range []string{"asset", "domain"} {
		t.Run(kind, func(t *testing.T) {
			h := newIdentityHarness(t)
			f := h.fixtureProviders()
			h.login()
			h.selectIdentity(200)
			var room detail
			h.call("POST", "/api/rooms", map[string]string{"name": "failure"}, 201, &room)
			u, _ := url.Parse(h.server.URL)
			headers := http.Header{"Origin": []string{h.server.URL}}
			for _, c := range h.client.Jar.Cookies(u) {
				headers.Add("Cookie", c.String())
			}
			ws, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.server.URL, "http")+"/api/rooms/"+room.Room.ID+"/ws", headers)
			if e != nil {
				t.Fatal(e)
			}
			defer ws.Close()
			old := h.app.store.path
			blocked := filepath.Join(t.TempDir(), "file")
			os.WriteFile(blocked, []byte("blocked"), 0600)
			h.app.store.mu.Lock()
			h.app.store.path = filepath.Join(blocked, "save.json")
			h.app.store.mu.Unlock()
			if kind == "asset" {
				f.assetOwned = false
			} else {
				f.nameOwned = false
			}
			h.selectIdentity(503)
			h.app.store.mu.Lock()
			h.app.store.path = old
			h.app.store.mu.Unlock()
			h.call("GET", "/api/rooms", nil, 401, nil)
			h.call("GET", "/api/identity/me", nil, 401, nil)
			ws.SetReadDeadline(time.Now().Add(time.Second))
			for {
				_, _, e = ws.ReadMessage()
				if e != nil {
					break
				}
			}
			if ne, ok := e.(interface{ Timeout() bool }); ok && ne.Timeout() {
				t.Fatal("failed persistence left socket active")
			}
		})
	}
}
func TestSelectedCharacterGameplayAndPersistence(t *testing.T) {
	h := newIdentityHarness(t)
	h.fixtureProviders()
	h.login()
	identity := h.selectIdentity(200)
	r, c := gatheringFixture()
	c.user = identityUser(identity)
	c.player.ID = identity.ID
	c.player.BtcName = identity.BtcName
	c.player.Username = identity.BtcName
	c.player.CharacterIdentityID = identity.ID
	r.record.Room = Room{ID: newID(), Code: "IDPLAY", OwnerID: identity.ID}
	r.record.Members = map[string]bool{identity.ID: true}
	save := func(record RoomRecord) error { return h.app.store.saveRoom(context.Background(), record) }
	if e := r.joinLife(c, 1000, save); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 3; n++ {
		at := int64(2000 + n*1000)
		action := string(rune('a' + n))
		if e := r.startGather(c, "tree-1", "basic-axe", action, at); e != nil {
			t.Fatal(e)
		}
		if e := r.hitGather(c, "tree-1", "basic-axe", action, at+280, save); e != nil {
			t.Fatal(e)
		}
	}
	c.player.Position = Vec2{0, 10}
	r.record.State.FreeTower.Inventory["wood"] = 100
	r.record.State.FreeTower.Inventory["stone"] = 100
	result, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: "identity-camp", DefinitionID: "campfire", Position: Vec2{4, 10}}, 10000, save)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.advanceBuildings(15000, false, nil, save); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Buildings[result.BuildingID].OwnerID != identity.ID {
		t.Fatal("building owner is not immutable character ID")
	}
	r.record.State.FreeTower.Buildings["identity-plot"] = BuildingInstance{ID: "identity-plot", DefinitionID: "farm-plot", Status: "completed", Position: BuildingPosition{Z: 18}}
	c.player.Position = Vec2{0, 18}
	if e = r.advanceLife(16000, false, nil, save); e != nil {
		t.Fatal(e)
	}
	run := func(q LifeRequest, at int64) {
		t.Helper()
		if e := r.lifeCommand(c, q, at, save); e != nil {
			t.Fatal(e)
		}
	}
	equipLife(r, c, "basic-hoe")
	run(LifeRequest{RequestID: "identity-till", Action: "till", TargetID: "identity-plot"}, 17000)
	run(LifeRequest{RequestID: "identity-plant", Action: "plant", TargetID: "identity-plot", ItemID: "wheat-seed"}, 18000)
	equipLife(r, c, "basic-watering-can")
	run(LifeRequest{RequestID: "identity-water", Action: "water", TargetID: "identity-plot"}, 19000)
	c.player.Position = r.record.State.FreeTower.Life.Animals["chicken-1"].Position
	run(LifeRequest{RequestID: "identity-feed", Action: "feed", TargetID: "chicken-1", ItemID: "chicken-feed"}, 20000)
	run(LifeRequest{RequestID: "identity-eat", Action: "eat", ItemID: "carrot"}, 21000)
	c.player.Position = Vec2{0, 3}
	c.player.Facing = 3.141592653589793
	equipLife(r, c, "basic-fishing-rod")
	run(LifeRequest{RequestID: "identity-fish", Action: "cast", TargetID: "silver-river", Position: &Vec2{0, -4}}, 22000)
	saved, e := openStore("", h.path)
	if e != nil {
		t.Fatal(e)
	}
	defer saved.close()
	rooms, e := saved.loadRooms(context.Background())
	if e != nil || len(rooms) != 1 {
		t.Fatal("gameplay room not persisted")
	}
	state := rooms[0].State.FreeTower
	personal := state.Personal[identity.ID]
	if personal.Owner.UserID != identity.ID || personal.Inventory["carrot"] != 2 || personal.Inventory["wheat-seed"] != 4 || personal.Inventory["basic-bait"] != 19 {
		t.Fatalf("character inventory mapping lost: %+v", personal.Inventory)
	}
	if _, exists := state.Personal[h.address]; exists {
		t.Fatal("wallet address used as inventory key")
	}
	if !liveFishing(state.Life.Fishing[identity.ID]) || !rooms[0].Members[identity.ID] {
		t.Fatal("fishing or membership not keyed to character")
	}
}
