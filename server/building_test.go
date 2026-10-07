package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func buildingFixture() (*RuntimeRoom, *Client) {
	r, c := gatheringFixture()
	c.player.Position = Vec2{0, 10}
	r.record.State.FreeTower.Inventory = map[string]int{"wood": 100, "stone": 100}
	return r, c
}
func TestBuildingDurabilityRollbackAndMove(t *testing.T) {
	r, c := buildingFixture()
	save := func(RoomRecord) error { return nil }
	req := BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}
	before := cloneState(r.record.State)
	if _, e := r.buildingCommand(c, "place-building", req, 10000, func(RoomRecord) error { return errors.New("disk") }); e == nil || !reflect.DeepEqual(before, r.record.State) {
		t.Fatal("failed save changed state")
	}
	res, e := r.buildingCommand(c, "place-building", req, 10000, save)
	if e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("cost")
	}
	if _, e = r.buildingCommand(c, "place-building", req, 10001, save); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("replay charged")
	}
	req.Position.X++
	if _, e = r.buildingCommand(c, "place-building", req, 11000, save); e == nil {
		t.Fatal("changed payload replay")
	}
	if e = r.advanceBuildings(15000, false, nil, save); e != nil {
		t.Fatal(e)
	}
	b := r.record.State.FreeTower.Buildings[res.BuildingID]
	if b.Status != "completed" {
		t.Fatal(b)
	}
	move, e := r.buildingCommand(c, "begin-building-move", BuildingRequest{RequestID: "b", BuildingID: b.ID}, 16000, save)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.buildingCommand(c, "move-building", BuildingRequest{RequestID: "m", BuildingID: b.ID, MoveToken: move.MoveToken, Position: Vec2{7, 10}}, 17000, save); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Buildings[b.ID].ConstructionEndsAt != b.ConstructionEndsAt {
		t.Fatal("timer reset")
	}
	if buildingMovement(r.record.State.FreeTower, Vec2{0, 10}, Vec2{12, 10}) != (Vec2{0, 10}) {
		t.Fatal("crossed building")
	}
}

func TestBuildingValidationClasses(t *testing.T) {
	cases := []struct {
		name     string
		p        Vec2
		rotation float64
		setup    func(*RuntimeRoom, *Client)
	}{{"bounds", Vec2{410, 10}, 0, nil}, {"protected", Vec2{16, 340}, 0, nil}, {"water", Vec2{0, 72}, 0, nil}, {"range", Vec2{70, 10}, 0, nil}, {"rotation", Vec2{4, 10}, .1, nil}, {"player", Vec2{0, 10}, 0, nil}, {"inventory", Vec2{4, 10}, 0, func(r *RuntimeRoom, c *Client) { r.record.State.FreeTower.Inventory["wood"] = 0 }}, {"resource", Vec2{4, 10}, 0, func(r *RuntimeRoom, c *Client) {
		r.record.State.FreeTower.Nodes["test"] = ResourceNodeState{DefinitionID: "rock-basic", Position: Vec2{4, 10}, Scale: 1, Depleted: true}
	}}, {"overlap", Vec2{4, 10}, 0, func(r *RuntimeRoom, c *Client) {
		r.record.State.FreeTower.Buildings = map[string]BuildingInstance{"b": {ID: "b", DefinitionID: "small-shelter", Position: BuildingPosition{5, 2, 10}, RotationY: math.Pi / 6, Status: "constructing"}}
	}}, {"swimming", Vec2{4, 10}, 0, func(r *RuntimeRoom, c *Client) { c.player.MovementMode = "swimming" }}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r, c := buildingFixture()
			if tt.setup != nil {
				tt.setup(r, c)
			}
			_, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: tt.p, RotationY: tt.rotation}, 10000, func(RoomRecord) error { return nil })
			if e == nil {
				t.Fatal("accepted invalid placement")
			}
			if ge, ok := e.(*gatherError); !ok || ge.Code != tt.name {
				t.Fatalf("wanted %s got %v", tt.name, e)
			}
		})
	}
}
func TestBuildingLocksRollbackRecovery(t *testing.T) {
	for _, recovery := range []string{"cancel", "expiry", "disconnect", "restart"} {
		t.Run(recovery, func(t *testing.T) {
			r, c := buildingFixture()
			save := func(RoomRecord) error { return nil }
			res, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}, 10000, save)
			if e != nil {
				t.Fatal(e)
			}
			r.advanceBuildings(15000, false, nil, save)
			q := BuildingRequest{RequestID: "b", BuildingID: res.BuildingID}
			lock, e := r.buildingCommand(c, "begin-building-move", q, 16000, save)
			if e != nil {
				t.Fatal(e)
			}
			before := cloneState(r.record.State)
			other := &Client{user: c.user, player: c.player, send: make(chan []byte, 100)}
			r.clients[other] = true
			if _, e = r.buildingCommand(other, "cancel-building-move", BuildingRequest{BuildingID: res.BuildingID, MoveToken: lock.MoveToken}, 17000, save); e == nil {
				t.Fatal("other session used token")
			}
			fail := func(RoomRecord) error { return errors.New("disk") }
			if recovery == "cancel" {
				q = BuildingRequest{BuildingID: res.BuildingID, MoveToken: lock.MoveToken}
				if _, e = r.buildingCommand(c, "cancel-building-move", q, 17000, fail); e == nil {
					t.Fatal("save accepted")
				}
				if !reflect.DeepEqual(before, r.record.State) {
					t.Fatal("failed cancel changed state")
				}
				_, e = r.buildingCommand(c, "cancel-building-move", q, 17000, save)
			} else {
				at := int64(17000)
				if recovery == "expiry" {
					at = 46000
				}
				var disconnect *Client
				if recovery == "disconnect" {
					disconnect = c
				}
				if e = r.advanceBuildings(at, recovery == "restart", disconnect, fail); e == nil {
					t.Fatal("save accepted")
				}
				if !reflect.DeepEqual(before, r.record.State) {
					t.Fatal("failed recovery changed state")
				}
				e = r.advanceBuildings(at, recovery == "restart", disconnect, save)
			}
			if e != nil {
				t.Fatal(e)
			}
			b := r.record.State.FreeTower.Buildings[res.BuildingID]
			if b.Status != "completed" || b.Position.X != 4 || b.MoveOwnerID != "" {
				t.Fatal(b)
			}
			if _, e = r.buildingCommand(c, "move-building", BuildingRequest{RequestID: "m", BuildingID: b.ID, MoveToken: lock.MoveToken, Position: Vec2{7, 10}}, 47000, save); e == nil {
				t.Fatal("stale token")
			}
		})
	}
}
func TestBuildingConcurrentOverlap(t *testing.T) {
	r, c := buildingFixture()
	var wg sync.WaitGroup
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.mu.Lock()
			defer r.mu.Unlock()
			_, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: fmt.Sprint(i), DefinitionID: "campfire", Position: Vec2{4, 10}}, 10000+int64(i)*1000, func(RoomRecord) error { return nil })
			if e == nil {
				success++
			}
		}(i)
	}
	wg.Wait()
	if success != 1 || len(r.record.State.FreeTower.Buildings) != 1 || r.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("not serialized")
	}
}
func TestBuildingRestartStorage(t *testing.T) {
	store, e := openStore("", filepath.Join(t.TempDir(), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.close()
	r, c := buildingFixture()
	r.record.Room.ID = newID()
	save := func(rec RoomRecord) error { return store.saveRoom(context.Background(), rec) }
	q := BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}
	res, e := r.buildingCommand(c, "place-building", q, 10000, save)
	if e != nil {
		t.Fatal(e)
	}
	records, e := store.loadRooms(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(records) != 1 {
		t.Fatal(records)
	}
	r.record = records[0]
	if _, e = r.buildingCommand(c, "place-building", q, 12000, save); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("replay charged")
	}
	if e = r.advanceBuildings(15000, true, nil, save); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Buildings[res.BuildingID].Status != "completed" {
		t.Fatal("not completed")
	}
}
func TestBuildingPostgresRestart(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "ft_building_test_" + newID()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	parsed, e := url.Parse(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	store, e := openStore(parsed.String(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer store.close()
	r, c := buildingFixture()
	r.record.Room = Room{ID: newID(), Code: newID(), OwnerID: c.user.ID}
	save := func(rec RoomRecord) error { return store.saveRoom(ctx, rec) }
	q := BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}
	res, e := r.buildingCommand(c, "place-building", q, nowMS(), save)
	if e != nil {
		t.Fatal(e)
	}
	fresh, e := openStore(parsed.String(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.close()
	app, e := newApp(fresh, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer app.shutdown()
	restored := app.rooms[r.record.Room.ID]
	restored.mu.Lock()
	defer restored.mu.Unlock()
	restored.clients[c] = true
	defer delete(restored.clients, c)
	if _, e = restored.buildingCommand(c, "place-building", q, nowMS(), save); e != nil {
		t.Fatal(e)
	}
	if restored.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("restart double cost")
	}
	b := restored.record.State.FreeTower.Buildings[res.BuildingID]
	if e = restored.advanceBuildings(b.ConstructionEndsAt, false, nil, save); e != nil {
		t.Fatal(e)
	}
	lock, e := restored.buildingCommand(c, "begin-building-move", BuildingRequest{RequestID: "b", BuildingID: b.ID}, b.ConstructionEndsAt+1000, save)
	if e != nil {
		t.Fatal(e)
	}
	records, e := fresh.loadRooms(ctx)
	if e != nil {
		t.Fatal(e)
	}
	rr := &RuntimeRoom{record: records[0], clients: map[*Client]bool{c: true}}
	if e = rr.advanceBuildings(b.ConstructionEndsAt+2000, true, nil, save); e != nil {
		t.Fatal(e)
	}
	if rr.record.State.FreeTower.Buildings[b.ID].Status != "completed" {
		t.Fatal("restart orphaned lock")
	}
	if _, e = rr.buildingCommand(c, "move-building", BuildingRequest{RequestID: "m", BuildingID: b.ID, MoveToken: lock.MoveToken, Position: Vec2{7, 10}}, b.ConstructionEndsAt+3000, save); e == nil {
		t.Fatal("restart token still accepted")
	}
}
func TestBuildingPrivateReceiptsAndGatherClone(t *testing.T) {
	r, c := buildingFixture()
	_, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}, 10000, func(RoomRecord) error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(publicState(r.record.State))
	if e != nil || bytes.Contains(raw, []byte("buildingReceipts")) {
		t.Fatal("private receipt exposed")
	}
	clone := cloneState(r.record.State)
	clone.FreeTower.Inventory["wood"]++
	if len(clone.FreeTower.Buildings) != 1 || len(clone.FreeTower.BuildingReceipts) != 1 || r.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("clone lost buildings/receipts")
	}
}
func TestBuildingRotatedFootprints(t *testing.T) {
	d := buildingDefinition("small-shelter")
	a := math.Pi / 6
	p := Vec2{10, 10}
	b := BuildingInstance{DefinitionID: "workbench", Position: BuildingPosition{12, 2, 10}, RotationY: -a}
	if !buildingOverlap(p, a, d, b) {
		t.Fatal("rotated overlap missed")
	}
	b.Position.X = 20
	if buildingOverlap(p, a, d, b) {
		t.Fatal("far overlap")
	}
	for _, a := range []float64{0, math.Pi / 6, math.Pi / 2} {
		corner := buildingPoint(p, a, d.Footprint.Width/2, d.Footprint.Depth/2)
		if !buildingCircle(p, a, d, corner, .01) {
			t.Fatal("rotated corner missed")
		}
	}
}
func TestBuildingSlopeHeightAndTree(t *testing.T) {
	for _, code := range []string{"slope", "height-delta", "tree"} {
		t.Run(code, func(t *testing.T) {
			r, c := buildingFixture()
			d := *buildingDefinition("small-shelter")
			if code == "height-delta" {
				d.PlacementRules.MaxSlope = 100
			}
			found := false
			for x := -70.; x <= 70 && !found; x += 1 {
				for z := -80.; z <= 25; z += 1 {
					p := Vec2{x, z}
					c.player.Position = Vec2{x + 7, z}
					e := r.validateBuilding(c, &d, p, math.Pi/6, "", true)
					if ge, ok := e.(*gatherError); ok && ge.Code == code {
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatal("no rejection found", code)
			}
		})
	}
}
func TestBuildingConstructionFailedSaveAndBeginReplay(t *testing.T) {
	r, c := buildingFixture()
	save := func(RoomRecord) error { return nil }
	res, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}, 10000, save)
	if e != nil {
		t.Fatal(e)
	}
	before := cloneState(r.record.State)
	if e = r.advanceBuildings(15000, false, nil, func(RoomRecord) error { return errors.New("disk") }); e == nil || !reflect.DeepEqual(before, r.record.State) {
		t.Fatal("construction published failed save")
	}
	r.advanceBuildings(15000, false, nil, save)
	q := BuildingRequest{RequestID: "b", BuildingID: res.BuildingID}
	first, e := r.buildingCommand(c, "begin-building-move", q, 16000, save)
	if e != nil {
		t.Fatal(e)
	}
	second, e := r.buildingCommand(c, "begin-building-move", q, 16001, save)
	if e != nil || first != second {
		t.Fatal("begin replay failed", e)
	}
	before = cloneState(r.record.State)
	if _, e = r.buildingCommand(c, "move-building", BuildingRequest{RequestID: "m", BuildingID: res.BuildingID, MoveToken: first.MoveToken, Position: Vec2{7, 10}}, 17000, func(RoomRecord) error { return errors.New("disk") }); e == nil || !reflect.DeepEqual(before, r.record.State) {
		t.Fatal("failed move changed state")
	}
}
func TestBuildingInvalidMoveRestoresOriginal(t *testing.T) {
	r, c := buildingFixture()
	save := func(RoomRecord) error { return nil }
	p, e := r.buildingCommand(c, "place-building", BuildingRequest{RequestID: "p", DefinitionID: "campfire", Position: Vec2{4, 10}}, 10000, save)
	if e != nil {
		t.Fatal(e)
	}
	r.advanceBuildings(15000, false, nil, save)
	lock, e := r.buildingCommand(c, "begin-building-move", BuildingRequest{RequestID: "b", BuildingID: p.BuildingID}, 16000, save)
	if e != nil {
		t.Fatal(e)
	}
	q := BuildingRequest{RequestID: "m", BuildingID: p.BuildingID, MoveToken: lock.MoveToken, Position: Vec2{0, 10}}
	before := cloneState(r.record.State)
	if _, e = r.buildingCommand(c, "move-building", q, 17000, func(RoomRecord) error { return errors.New("disk") }); e == nil || !reflect.DeepEqual(before, r.record.State) {
		t.Fatal("invalid move failed save mutated state")
	}
	if _, e = r.buildingCommand(c, "move-building", q, 17001, save); e == nil {
		t.Fatal("invalid move accepted")
	}
	b := r.record.State.FreeTower.Buildings[p.BuildingID]
	if b.Status != "completed" || b.Position.X != 4 || r.record.State.FreeTower.Inventory["wood"] != 96 {
		t.Fatal("invalid move did not restore", b)
	}
}
func TestBuildingOccupiedCompletionAllowsExit(t *testing.T) {
	f := &FreeTowerState{Buildings: map[string]BuildingInstance{"b": {ID: "b", DefinitionID: "small-shelter", Position: BuildingPosition{0, 2, 0}, Status: "completed"}}}
	old := Vec2{.5, 0}
	p := Vec2{5, 0}
	if buildingMovement(f, old, p) != p {
		t.Fatal("player trapped on completion")
	}
	if buildingMovement(f, old, Vec2{0, 0}) != old {
		t.Fatal("allowed deeper movement")
	}
}
func TestBuildingLegacyResourceScale(t *testing.T) {
	r, c := buildingFixture()
	node := gatheringConfig.Nodes[5]
	if node.ID != "rock-1" {
		t.Fatal("canonical fixture changed")
	}
	legacy := ResourceNodeState{ID: node.ID, DefinitionID: node.DefinitionID, Position: node.Position, Depleted: true}
	raw, _ := json.Marshal(legacy)
	var legacyJSON map[string]any
	_ = json.Unmarshal(raw, &legacyJSON)
	delete(legacyJSON, "scale")
	raw, _ = json.Marshal(legacyJSON)
	var restored ResourceNodeState
	if e := json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	r.record.State.FreeTower.Nodes = map[string]ResourceNodeState{node.ID: restored}
	rr := &RuntimeRoom{record: r.record, clients: r.clients}
	rr.ensureGathering()
	if rr.record.State.FreeTower.Nodes[node.ID].Scale != 0 {
		t.Fatal("fixture not legacy")
	}
	d := *buildingDefinition("campfire")
	d.PlacementRules.MaxSlope = 100
	d.PlacementRules.MaxHeightDelta = 100
	p := Vec2{node.Position.X + 2.3, node.Position.Z}
	c.player.Position = Vec2{p.X + 4, p.Z}
	e := rr.validateBuilding(c, &d, p, 0, "", false)
	if ge, ok := e.(*gatherError); !ok || ge.Code != "resource" {
		t.Fatalf("legacy radius not canonical: %v", e)
	}
	canonical := rr.record.State.FreeTower.Nodes[node.ID]
	canonical.Scale = node.Scale
	rr.record.State.FreeTower.Nodes[node.ID] = canonical
	e = rr.validateBuilding(c, &d, p, 0, "", false)
	if ge, ok := e.(*gatherError); !ok || ge.Code != "resource" {
		t.Fatalf("canonical exclusion mismatch: %v", e)
	}
}
