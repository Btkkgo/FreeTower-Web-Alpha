package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestLifeRollbackReplayPrivacy(t *testing.T) {
	r, c := gatheringFixture()
	save := func(RoomRecord) error { return nil }
	if e := r.joinLife(c, 1000, save); e != nil {
		t.Fatal(e)
	}
	q := LifeRequest{RequestID: "eat1", Action: "eat", ItemID: "carrot"}
	if e := r.lifeCommand(c, q, 2000, func(RoomRecord) error { return errors.New("disk") }); e == nil {
		t.Fatal("save accepted")
	}
	if r.record.State.FreeTower.Personal[c.user.ID].Inventory["carrot"] != 3 {
		t.Fatal("rollback")
	}
	for i := 0; i < 2; i++ {
		if e := r.lifeCommand(c, q, 2000, save); e != nil {
			t.Fatal(e)
		}
	}
	if r.record.State.FreeTower.Personal[c.user.ID].Inventory["carrot"] != 2 {
		t.Fatal("replay")
	}
	b, _ := json.Marshal(publicState(r.record.State))
	if strings.Contains(string(b), "survival") || strings.Contains(string(b), "eat1") {
		t.Fatal("private leak")
	}
}
func lifeFixture(t *testing.T) (*RuntimeRoom, *Client) {
	t.Helper()
	r, c := gatheringFixture()
	c.player.Position = Vec2{0, 18}
	r.record.Room.ID = "world-one"
	if e := r.joinLife(c, 1000, func(RoomRecord) error { return nil }); e != nil {
		t.Fatal(e)
	}
	return r, c
}
func lifeOK(RoomRecord) error { return nil }
func equipLife(r *RuntimeRoom, c *Client, id string) {
	u := r.gatheringUser(c)
	u.Hands.Right = id
	u.Hands.Active = "right"
	r.syncGatherUser(c.user.ID)
}
func TestLifeFishingAuthority(t *testing.T) {
	r, c := lifeFixture(t)
	c.player.Position = Vec2{0, 3}
	c.player.Facing = 3.141592653589793
	q := LifeRequest{RequestID: "cast", Action: "cast", TargetID: "silver-river", Position: &Vec2{0, -4}}
	if e := r.lifeCommand(c, q, 2000, lifeOK); e == nil {
		t.Fatal("rodless cast")
	}
	equipLife(r, c, "basic-fishing-rod")
	if e := r.lifeCommand(c, q, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	if e := r.lifeCommand(c, q, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Personal["u"].Inventory["basic-bait"] != 19 {
		t.Fatal("bait replay")
	}
	s := r.record.State.FreeTower.Life.Fishing["u"]
	if e := r.advanceLife(s.BiteAt-100, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Life.Fishing["u"].Distance != 12 {
		t.Fatal("early fight")
	}
	for i := 0; i < 20; i++ {
		if e := r.lifeCommand(c, LifeRequest{RequestID: fmt.Sprintf("reel%d", i), Action: "reel", Reeling: true}, s.BiteAt, lifeOK); e != nil {
			t.Fatal(e)
		}
	}
	if r.record.State.FreeTower.Life.Fishing["u"].Distance != 12 {
		t.Fatal("input frequency progressed")
	}
	for i := 0; i < 6; i++ {
		f := r.record.State.FreeTower.Life.Fishing["u"]
		f.Tension = 0
		r.record.State.FreeTower.Life.Fishing["u"] = f
		if e := r.advanceLife(s.BiteAt+int64(i+1)*1000, false, nil, lifeOK); e != nil {
			t.Fatal(e)
		}
	}
	p := r.record.State.FreeTower.Personal["u"]
	if p.Inventory[s.FishID] != 1 {
		t.Fatal("catch reward")
	}
	if e := r.advanceLife(s.BiteAt+7000, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Personal["u"].Inventory[s.FishID] != 1 {
		t.Fatal("duplicate catch")
	}
}
func TestLifeFarmEggSurvivalReload(t *testing.T) {
	r, c := lifeFixture(t)
	r.record.State.FreeTower.Buildings = map[string]BuildingInstance{"plot": {ID: "plot", DefinitionID: "farm-plot", Status: "completed", Position: BuildingPosition{X: 0, Z: 18}}}
	if e := r.advanceLife(2000, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	run := func(id, action, target, item string, at int64) {
		t.Helper()
		if e := r.lifeCommand(c, LifeRequest{RequestID: id, Action: action, TargetID: target, ItemID: item}, at, lifeOK); e != nil {
			t.Fatal(e)
		}
	}
	if e := r.lifeCommand(c, LifeRequest{RequestID: "nohoe", Action: "till", TargetID: "plot"}, 2000, lifeOK); e == nil {
		t.Fatal("hoe required")
	}
	equipLife(r, c, "basic-hoe")
	run("till", "till", "plot", "", 2000)
	run("plant", "plant", "plot", "wheat-seed", 3000)
	run("plant", "plant", "plot", "wheat-seed", 3000)
	if r.record.State.FreeTower.Personal["u"].Inventory["wheat-seed"] != 4 {
		t.Fatal("seed replay")
	}
	equipLife(r, c, "basic-watering-can")
	run("water", "water", "plot", "", 4000)
	if e := r.lifeCommand(c, LifeRequest{RequestID: "early", Action: "harvest", TargetID: "plot"}, 5000, lifeOK); e == nil {
		t.Fatal("early harvest")
	}
	c.player.Position = r.record.State.FreeTower.Life.Animals["chicken-1"].Position
	run("feed", "feed", "chicken-1", "chicken-feed", 5000)
	c.player.Position = Vec2{0, 18}
	r.record.State = cloneState(r.record.State)
	if e := r.advanceLife(49000, true, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Life.Plots["plot"].State != "ready" {
		t.Fatal("restart crop")
	}
	run("harvest", "harvest", "plot", "", 49000)
	run("harvest", "harvest", "plot", "", 49000)
	a := r.record.State.FreeTower.Life.Animals["chicken-1"]
	if !a.EggReady || a.EggPosition == nil {
		t.Fatal("egg timing")
	}
	c.player.Position = *a.EggPosition
	run("egg", "collect-egg", "chicken-1", "", 49250)
	run("egg", "collect-egg", "chicken-1", "", 49250)
	if r.record.State.FreeTower.Personal["u"].Inventory["egg"] != 1 {
		t.Fatal("egg replay")
	}
	p := r.record.State.FreeTower.Personal["u"]
	if p.Survival.Hunger >= 100 || p.Survival.Thirst >= 100 {
		t.Fatal("decay")
	}
	p.Survival.Thirst = 60
	p.Survival.Hunger = 60
	r.record.State.FreeTower.Personal["u"] = p
	run("drink", "drink", "", "water-flask", 50000)
	run("eat", "eat", "", "carrot", 50250)
	if r.record.State.FreeTower.Personal["u"].Survival.Hunger != 78 || r.record.State.FreeTower.Personal["u"].Survival.Thirst != 85 {
		t.Fatal("restoration")
	}
}
func TestLifeIsolationRollbackRestart(t *testing.T) {
	r, c := lifeFixture(t)
	other := &Client{user: User{ID: "other"}, player: Player{ID: "other", World: "freetower", Position: Vec2{0, 18}}, send: make(chan []byte, 100)}
	r.clients[other] = true
	r.record.Members["other"] = true
	if e := r.joinLife(other, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	q := LifeRequest{RequestID: "own", Action: "eat", ItemID: "carrot"}
	if e := r.lifeCommand(c, q, 3000, lifeOK); e != nil {
		t.Fatal(e)
	}
	if e := r.lifeCommand(other, q, 3000, lifeOK); e == nil {
		t.Fatal("receipt conflict")
	}
	if r.record.State.FreeTower.Personal["other"].Inventory["carrot"] != 3 {
		t.Fatal("pooled inventory")
	}
	old := cloneState(r.record.State)
	if e := r.advanceLife(60000, false, nil, func(RoomRecord) error { return errors.New("disk") }); e == nil {
		t.Fatal("timer save")
	}
	if !reflect.DeepEqual(old, r.record.State) {
		t.Fatal("timer rollback")
	}
	f := r.record.State.FreeTower
	f.Life.Fishing["u"] = FishingSession{ID: "live", UserID: "u", State: "waiting", UpdatedAt: 2000}
	if e := r.advanceLife(4000, true, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Life.Fishing["u"].State != "cancelled" {
		t.Fatal("restart fishing")
	}
	if e := r.joinLife(c, 5000, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Personal["u"].Inventory["carrot"] != 2 {
		t.Fatal("starter duplicated")
	}
}
func TestLifeAnimalObstaclesPlotWalkable(t *testing.T) {
	r, _ := lifeFixture(t)
	f := r.record.State.FreeTower
	if animalAllowed(f, Vec2{0, 18}, Vec2{0, -4}) {
		t.Fatal("water accepted")
	}
	f.Buildings = map[string]BuildingInstance{"plot": {ID: "plot", DefinitionID: "farm-plot", Status: "completed", Position: BuildingPosition{X: 0, Z: 18}}}
	if buildingMovement(f, Vec2{-3, 18}, Vec2{0, 18}) != (Vec2{0, 18}) {
		t.Fatal("plot not walkable")
	}
	for _, d := range buildingConfig.Definitions {
		if d.ID != "farm-plot" {
			f.Buildings["wall"] = BuildingInstance{ID: "wall", DefinitionID: d.ID, Status: "completed", Position: BuildingPosition{X: 0, Z: 18}}
			if animalAllowed(f, Vec2{-8, 18}, Vec2{0, 18}) {
				t.Fatal("building collision ignored")
			}
			break
		}
	}
}
func TestLifeFishingCancelAndEscape(t *testing.T) {
	for _, reason := range []string{"swimming", "movement", "tool", "disconnect", "timeout", "tension"} {
		t.Run(reason, func(t *testing.T) {
			r, c := lifeFixture(t)
			equipLife(r, c, "basic-fishing-rod")
			c.player.Position = Vec2{0, 3}
			s := FishingSession{ID: "cast", UserID: "u", State: "hooked", FishID: "river-minnow", Origin: c.player.Position, BiteAt: 2000, ExpiresAt: 62000, Distance: 12, UpdatedAt: 2000}
			r.record.State.FreeTower.Life.Fishing["u"] = s
			at := int64(3000)
			var disconnected *Client
			switch reason {
			case "swimming":
				c.player.Position = Vec2{0, -4}
			case "movement":
				c.player.Position = Vec2{3, 3}
			case "tool":
				equipLife(r, c, "basic-hoe")
			case "disconnect":
				disconnected = c
			case "timeout":
				at = 62000
			case "tension":
				s.Tension = 99
				s.Reeling = true
				r.record.State.FreeTower.Life.Fishing["u"] = s
			}
			if e := r.advanceLife(at, false, disconnected, lifeOK); e != nil {
				t.Fatal(e)
			}
			if liveFishing(r.record.State.FreeTower.Life.Fishing["u"]) {
				t.Fatal("session still active")
			}
			if r.record.State.FreeTower.Personal["u"].Inventory["river-minnow"] != 0 {
				t.Fatal("escaped reward")
			}
		})
	}
}
func TestLifeInvalidCasts(t *testing.T) {
	cases := []struct {
		name   string
		tool   string
		player Vec2
		target Vec2
		spot   string
		facing float64
	}{{"without-rod", "basic-hoe", Vec2{0, 3}, Vec2{0, -4}, "silver-river", math.Pi}, {"swimming", "basic-fishing-rod", Vec2{0, -4}, Vec2{0, -8}, "silver-river", math.Pi}, {"land-target", "basic-fishing-rod", Vec2{0, 18}, Vec2{0, 10}, "silver-river", math.Pi}, {"wrong-spot", "basic-fishing-rod", Vec2{0, 3}, Vec2{0, -4}, "ancient-sea", math.Pi}, {"too-near", "basic-fishing-rod", Vec2{0, 3}, Vec2{0, 2}, "silver-river", math.Pi}, {"too-far", "basic-fishing-rod", Vec2{0, 18}, Vec2{0, -4}, "silver-river", math.Pi}, {"backwards", "basic-fishing-rod", Vec2{0, 3}, Vec2{0, -4}, "silver-river", 0}, {"nonfinite", "basic-fishing-rod", Vec2{0, 3}, Vec2{math.Inf(1), -4}, "silver-river", math.Pi}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, c := lifeFixture(t)
			equipLife(r, c, tc.tool)
			c.player.Position = tc.player
			c.player.Facing = tc.facing
			e := r.lifeCommand(c, LifeRequest{RequestID: "bad", Action: "cast", TargetID: tc.spot, Position: &tc.target}, 2000, lifeOK)
			if e == nil {
				t.Fatal("invalid cast accepted")
			}
			if r.record.State.FreeTower.Personal["u"].Inventory["basic-bait"] != 20 {
				t.Fatal("invalid cast consumed bait")
			}
		})
	}
}
func TestLifeInvalidFarmActions(t *testing.T) {
	for _, action := range []string{"plant", "water", "harvest"} {
		t.Run(action, func(t *testing.T) {
			r, c := lifeFixture(t)
			r.record.State.FreeTower.Buildings = map[string]BuildingInstance{"plot": {ID: "plot", DefinitionID: "farm-plot", Status: "completed", Position: BuildingPosition{Z: 18}}}
			if e := r.advanceLife(2000, false, nil, lifeOK); e != nil {
				t.Fatal(e)
			}
			equipLife(r, c, "basic-watering-can")
			if e := r.lifeCommand(c, LifeRequest{RequestID: action, Action: action, TargetID: "plot", ItemID: "wheat-seed"}, 2000, lifeOK); e == nil {
				t.Fatal("invalid plot state")
			}
		})
	}
}
func TestLifePrivateDelivery(t *testing.T) {
	r, c := lifeFixture(t)
	other := &Client{user: User{ID: "other"}, player: Player{ID: "other", World: "freetower", Position: Vec2{0, 18}}, send: make(chan []byte, 100)}
	r.clients[other] = true
	r.record.Members["other"] = true
	if e := r.joinLife(other, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	for len(c.send) > 0 {
		<-c.send
	}
	for len(other.send) > 0 {
		<-other.send
	}
	r.sendLifeSelf(3000)
	for _, client := range []*Client{c, other} {
		var msg struct {
			Type     string
			Personal PersonalState
		}
		if e := json.Unmarshal(<-client.send, &msg); e != nil {
			t.Fatal(e)
		}
		if msg.Type != "life-self" || msg.Personal.Owner.UserID != client.user.ID {
			t.Fatal("wrong private owner")
		}
	}
}
func TestLifePlotMovePreservesCrop(t *testing.T) {
	r, c := lifeFixture(t)
	f := r.record.State.FreeTower
	f.Buildings = map[string]BuildingInstance{"plot": {ID: "plot", DefinitionID: "farm-plot", Status: "completed", Position: BuildingPosition{Z: 18}}}
	if e := r.advanceLife(2000, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	p := r.record.State.FreeTower.Life.Plots["plot"]
	p.CropID = "wheat"
	p.State = "growing"
	p.WateredAt = 2000
	p.ReadyAt = 47000
	r.record.State.FreeTower.Life.Plots["plot"] = p
	b := r.record.State.FreeTower.Buildings["plot"]
	b.Position.X = 10
	r.record.State.FreeTower.Buildings["plot"] = b
	if e := r.advanceLife(3000, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Life.Plots["plot"].CropID != "wheat" {
		t.Fatal("move lost crop")
	}
	if e := r.lifeCommand(c, LifeRequest{RequestID: "harvest", Action: "harvest", TargetID: "plot"}, 47000, lifeOK); e == nil {
		t.Fatal("old plot location accepted")
	}
}
func TestLifeEggStableAndDoubleFeed(t *testing.T) {
	r, c := lifeFixture(t)
	a := r.record.State.FreeTower.Life.Animals["chicken-1"]
	c.player.Position = a.Position
	q := LifeRequest{RequestID: "feed", Action: "feed", TargetID: a.ID, ItemID: "chicken-feed"}
	if e := r.lifeCommand(c, q, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	q.RequestID = "feed2"
	if e := r.lifeCommand(c, q, 3000, lifeOK); e == nil {
		t.Fatal("double feed")
	}
	if e := r.advanceLife(37000, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	a = r.record.State.FreeTower.Life.Animals[a.ID]
	egg := *a.EggPosition
	for i := 0; i < 8; i++ {
		if e := r.advanceLife(38000+int64(i)*1000, false, nil, lifeOK); e != nil {
			t.Fatal(e)
		}
	}
	if *r.record.State.FreeTower.Life.Animals[a.ID].EggPosition != egg {
		t.Fatal("egg moved")
	}
	c.player.Position = egg
	q = LifeRequest{RequestID: "collect", Action: "collect-egg", TargetID: a.ID}
	if e := r.lifeCommand(c, q, 46000, lifeOK); e != nil {
		t.Fatal(e)
	}
	q.RequestID = "collect-again"
	if e := r.lifeCommand(c, q, 47000, lifeOK); e == nil {
		t.Fatal("duplicate collection")
	}
}
func TestLifeConcurrentReplayAndRoomIsolation(t *testing.T) {
	r, c := lifeFixture(t)
	q := LifeRequest{RequestID: "concurrent", Action: "eat", ItemID: "carrot"}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.mu.Lock()
			defer r.mu.Unlock()
			if e := r.lifeCommand(c, q, 2000, lifeOK); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if r.record.State.FreeTower.Personal["u"].Inventory["carrot"] != 2 {
		t.Fatal("concurrent duplicated consume")
	}
	another, other := lifeFixture(t)
	another.record.Room.ID = "world-two"
	p := another.record.State.FreeTower.Personal["u"]
	p.Owner.RoomID = "world-two"
	another.record.State.FreeTower.Personal["u"] = p
	if other.user.ID != c.user.ID || p.Inventory["carrot"] != 3 {
		t.Fatal("character room isolation")
	}
	r.record.State.FreeTower.Personal["u"] = PersonalState{Inventory: map[string]int{"carrot": 0}, Survival: SurvivalState{Health: 100, Hunger: 1, Thirst: 1, Energy: 1, UpdatedAt: 1000}}
	if e := r.lifeCommand(c, LifeRequest{RequestID: "empty", Action: "eat", ItemID: "carrot"}, 2000, lifeOK); e == nil {
		t.Fatal("empty item accepted")
	}
	if e := r.advanceLife(601000, false, nil, lifeOK); e != nil {
		t.Fatal(e)
	}
	s := r.record.State.FreeTower.Personal["u"].Survival
	if s.Health != 100 || s.Hunger != 0 || s.Thirst != 0 || s.Energy <= 1 {
		t.Fatal("mild survival invariants")
	}
}
func TestLifeRateLimitNoopSafetyAndRollback(t *testing.T) {
	r, c := lifeFixture(t)
	saves := 0
	save := func(RoomRecord) error { saves++; return nil }
	q := LifeRequest{RequestID: "eat-rate", Action: "eat", ItemID: "carrot"}
	if e := r.lifeCommand(c, q, 2000, save); e != nil {
		t.Fatal(e)
	}
	if e := r.lifeCommand(c, q, 2001, save); e != nil {
		t.Fatal("durable replay throttled", e)
	}
	q.RequestID = "too-fast"
	if e := r.lifeCommand(c, q, 2002, save); e == nil {
		t.Fatal("rate limit missing")
	}
	if saves != 1 {
		t.Fatal("throttled write")
	}
	q.RequestID = "failed"
	if e := r.lifeCommand(c, q, 2250, func(RoomRecord) error { return errors.New("disk") }); e == nil {
		t.Fatal("failed write")
	}
	if e := r.lifeCommand(c, q, 2250, save); e != nil {
		t.Fatal("failed save spent budget", e)
	}
	equipLife(r, c, "basic-fishing-rod")
	f := r.record.State.FreeTower
	f.Life.Fishing["u"] = FishingSession{ID: "live", UserID: "u", State: "hooked", Reeling: true, Distance: 12, UpdatedAt: 2250}
	before := saves
	receipts := len(f.LifeReceipts)
	for i := 0; i < 300; i++ {
		if e := r.lifeCommand(c, LifeRequest{RequestID: fmt.Sprintf("noop-%d", i), Action: "reel", Reeling: true}, 2251, save); e != nil {
			t.Fatal(e)
		}
	}
	if saves != before || len(r.record.State.FreeTower.LifeReceipts) != receipts || len(r.lifeNoopReceipts) > 128 {
		t.Fatal("noop causes durable work/unbounded memory")
	}
	release := LifeRequest{RequestID: "release-safe", Action: "reel", Reeling: false}
	if e := r.lifeCommand(c, release, 2252, save); e != nil {
		t.Fatal("release throttled", e)
	}
	if r.record.State.FreeTower.Life.Fishing["u"].Reeling {
		t.Fatal("release ignored")
	}
	if e := r.lifeCommand(c, LifeRequest{RequestID: "enable-throttled", Action: "reel", Reeling: true}, 2253, save); e == nil {
		t.Fatal("enable not throttled")
	}
	if e := r.lifeCommand(c, LifeRequest{RequestID: "cancel-safe", Action: "cancel-fishing"}, 2254, save); e != nil {
		t.Fatal("cancel throttled", e)
	}
	if liveFishing(r.record.State.FreeTower.Life.Fishing["u"]) {
		t.Fatal("cancel ignored")
	}
}
func TestLifeReplayResyncRequesterOnlyBounded(t *testing.T) {
	r, c := lifeFixture(t)
	other := &Client{user: User{ID: "other"}, player: Player{ID: "other", World: "freetower", Position: Vec2{0, 18}}, send: make(chan []byte, 100)}
	r.clients[other] = true
	r.record.Members["other"] = true
	if e := r.joinLife(other, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	q := LifeRequest{RequestID: "replay-sync", Action: "eat", ItemID: "carrot"}
	if e := r.lifeCommand(c, q, 3000, lifeOK); e != nil {
		t.Fatal(e)
	}
	for len(c.send) > 0 {
		<-c.send
	}
	for len(other.send) > 0 {
		<-other.send
	}
	writes := 0
	save := func(RoomRecord) error { writes++; return nil }
	for i := 0; i < 100; i++ {
		if e := r.lifeCommand(c, q, 3001+int64(i), save); e != nil {
			t.Fatal(e)
		}
	}
	if len(other.send) != 0 {
		t.Fatal("replay touched other socket")
	}
	if len(c.send) != 1 || writes != 0 {
		t.Fatal("replay resync unbounded or wrote")
	}
	if e := r.lifeCommand(c, q, 3201, save); e != nil {
		t.Fatal(e)
	}
	if len(c.send) != 2 || len(other.send) != 0 {
		t.Fatal("resync rate recovery")
	}
}

func TestLifeReplayAckBound(t *testing.T) {
	r, c := lifeFixture(t)
	accepted := 0
	for i := 0; i < 100; i++ {
		if r.lifeReplayReplyAllowed(c, 1000+int64(i)) {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatal("replay acks unbounded")
	}
	if !r.lifeReplayReplyAllowed(c, 1200) {
		t.Fatal("replay ack did not recover")
	}
	other := &Client{}
	if !r.lifeReplayReplyAllowed(other, 1200) {
		t.Fatal("reply budget shared across sockets")
	}
}
