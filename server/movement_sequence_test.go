package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMovementPresenceAcknowledgesAcceptedAndCorrectedPacket(t *testing.T) {
	h := newHarness(t)
	client := h.client()
	h.signup(client, "MovementTester")
	var room detail
	h.request(client, "POST", "/api/rooms", map[string]string{"name": "Movement fixture"}, 201, &room)
	headers := http.Header{"Origin": []string{h.server.URL}}
	u, err := url.Parse(h.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range client.Jar.Cookies(u) {
		headers.Add("Cookie", c.String())
	}
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.server.URL, "http")+"/api/rooms/"+room.Room.ID+"/ws?world=freetower", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	readPresence := func(sequence uint64) Player {
		t.Helper()
		_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			var msg struct {
				Type    string   `json:"type"`
				Players []Player `json:"players"`
			}
			if err := ws.ReadJSON(&msg); err != nil {
				t.Fatal(err)
			}
			if msg.Type == "presence" && len(msg.Players) == 1 && msg.Players[0].MoveSequence == sequence {
				return msg.Players[0]
			}
		}
	}
	initial := readPresence(0)
	send := func(sequence uint64, p Vec2) {
		t.Helper()
		if err := ws.WriteJSON(map[string]any{"type": "move", "moveSequence": sequence, "position": p, "moving": true}); err != nil {
			t.Fatal(err)
		}
	}
	accepted := Vec2{initial.Position.X + .2, initial.Position.Z}
	send(1, accepted)
	if got := readPresence(1); got.Position != accepted {
		t.Fatal("accepted movement acknowledgement changed position")
	}
	time.Sleep(60 * time.Millisecond)
	send(2, Vec2{999, 999})
	corrected := readPresence(2)
	if corrected.Position.X == 999 || corrected.Position.Z == 999 {
		t.Fatal("movement sequence bypassed server speed/bounds correction")
	}
	time.Sleep(60 * time.Millisecond)
	// A replay must not reset the accepted sequence or consume the rate-limit slot
	// that belongs to this immediately following fresh packet.
	send(1, initial.Position)
	send(0, initial.Position)
	send(3, corrected.Position)
	if got := readPresence(3); got.Position != corrected.Position {
		t.Fatal("stale sequence rewound the authoritative player")
	}
}
