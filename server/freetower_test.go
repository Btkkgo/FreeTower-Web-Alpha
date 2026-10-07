package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFreeTowerShoreTransitions(t *testing.T) {
	for _, c := range []struct {
		z    float64
		mode string
	}{{38, "land"}, {44, "shallow-water"}, {50, "swimming"}, {44, "shallow-water"}, {38, "land"}} {
		mode, y := freeTowerLocomotion(Vec2{0, c.z})
		if mode != c.mode {
			t.Fatalf("z=%v mode=%s want=%s", c.z, mode, c.mode)
		}
		if mode == "swimming" && y < -1 {
			t.Fatal("swimmer sank")
		}
	}
}
func TestFreeTowerPresenceSerialization(t *testing.T) {
	p := Player{Position: Vec2{0, 50}, World: "freetower", MovementMode: "swimming", Moving: true, Elevation: -.35}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["movementMode"] != "swimming" || m["moving"] != true {
		t.Fatal(string(b))
	}
	if _, ok := m["position"].(map[string]any)["z"]; !ok {
		t.Fatal("lost position contract")
	}
}

// Two cookie jars and real sockets cover the whole shore journey, not just marshaling.
func TestFreeTowerWebsocketJourneyAndLegacyState(t *testing.T) {
	h := newHarness(t)
	a, b := h.client(), h.client()
	h.signup(a, "ExplorerA")
	h.signup(b, "ExplorerB")
	var room detail
	h.request(a, "POST", "/api/rooms", map[string]string{"name": "Ancient Coast"}, 201, &room)
	h.request(b, "POST", "/api/rooms/join", map[string]string{"code": room.Room.Code}, 200, nil)
	h.request(a, "POST", "/api/rooms/"+room.Room.ID+"/actions", Action{Type: "build", Facility: "shelter"}, 200, &room)
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
	go func() {
		for {
			if _, _, err := wa.ReadMessage(); err != nil {
				return
			}
		}
	}()
	readA := func(wantZ float64) Player {
		t.Helper()
		_ = wb.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			var m struct {
				Type    string
				Players []Player
			}
			if err := wb.ReadJSON(&m); err != nil {
				t.Fatal(err)
			}
			if m.Type == "presence" {
				for _, p := range m.Players {
					if p.Username == "ExplorerA" && math.Abs(p.Position.Z-wantZ) < .01 {
						return p
					}
				}
			}
		}
	}
	readA(18)
	observed := map[string]bool{}
	for _, direction := range []int{1, -1} {
		for i := 1; i <= 32; i++ {
			z := 18 + float64(i)
			if direction < 0 {
				z = 50 - float64(i)
			}
			time.Sleep(55 * time.Millisecond)
			if err := wa.WriteJSON(map[string]any{"type": "move", "position": Vec2{0, z}, "facing": .75, "moving": true, "running": true, "movementMode": "land"}); err != nil {
				t.Fatal(err)
			}
			p := readA(z)
			want, y := freeTowerLocomotion(Vec2{0, z})
			observed[p.MovementMode] = true
			if p.MovementMode != want || p.Elevation != y || !p.Moving || p.Facing != .75 {
				t.Fatalf("bad remote state: %+v", p)
			}
			if want == "swimming" && p.Running {
				t.Fatal("remote swimming rendered as running")
			}
		}
	}
	for _, mode := range []string{"land", "shallow-water", "swimming"} {
		if !observed[mode] {
			t.Fatal("missing mode", mode)
		}
	}
	var after detail
	h.request(a, "GET", "/api/rooms/"+room.Room.ID, nil, 200, &after)
	if after.State.Coins != room.State.Coins || !reflect.DeepEqual(after.State.Buildings, room.State.Buildings) || !reflect.DeepEqual(after.State.Inventory, room.State.Inventory) {
		t.Fatal("exploration changed legacy farm progress")
	}
}

func TestFreeTowerPresenceSeparatesLegacyCoordinates(t *testing.T) {
	rr := &RuntimeRoom{clients: map[*Client]bool{}}
	rr.clients[&Client{user: User{ID: "same"}, player: Player{ID: "same", World: "freetower", Position: Vec2{0, 18}}}] = true
	rr.clients[&Client{user: User{ID: "same"}, player: Player{ID: "same", Position: Vec2{1, 7}}}] = true
	if p := rr.playersInWorld("freetower"); len(p) != 1 || p[0].Position.Z != 18 {
		t.Fatal(p)
	}
	if p := rr.playersInWorld(""); len(p) != 1 || p[0].Position.Z != 7 {
		t.Fatal(p)
	}
}

func TestFreeTowerCannotSupplyLegacyGameplayPosition(t *testing.T) {
	rr := &RuntimeRoom{clients: map[*Client]bool{}}
	c := &Client{user: User{ID: "same"}, player: Player{ID: "same", World: "freetower", Position: worldMap.Meadow.GrazePoint}}
	rr.clients[c] = true
	if rr.playerPosition("same") != nil {
		t.Fatal("FreeTower connection must not drive legacy spatial gameplay")
	}
	legacy := &Client{user: User{ID: "same"}, player: Player{ID: "same", Position: Vec2{1, 7}}}
	rr.clients[legacy] = true
	for i := 0; i < 100; i++ {
		if p := rr.playerPosition("same"); p == nil || *p != legacy.player.Position {
			t.Fatal("mixed coordinate spaces", p)
		}
	}
}
