package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
)

//go:embed freetower_life.json
var lifeJSON []byte
var lifeConfig = func() LifeConfig {
	var c LifeConfig
	if e := json.Unmarshal(lifeJSON, &c); e != nil {
		panic(e)
	}
	return c
}()

type LifeConfig struct {
	InteractionRange float64        `json:"interactionRange"`
	StarterItems     map[string]int `json:"starterItems"`
	Survival         struct{ HungerDecay, ThirstDecay, EnergyRegeneration float64 }
	Fishing          struct{ MinRange, MaxRange, FacingConeDegrees, InitialDistance, ReelSpeed, TensionGain, TensionRelax, MaxTension, ExpirySeconds float64 }
	Fish             []FishDefinition
	Crops            []CropDefinition
	Animals          struct {
		Count                int
		Center               Vec2
		HabitatRadius, Speed float64
		EggSeconds           int64
	}
}
type FishDefinition struct {
	ID, AssetID, Habitat     string
	Rarity                   int
	BiteDelayRange           []float64
	FightStrength, BaseValue float64
}
type CropDefinition struct {
	ID, SeedID      string
	GrowthSeconds   int64
	HarvestQuantity int
}
type LifeRequest struct {
	RequestID string `json:"requestId"`
	Action    string `json:"action"`
	TargetID  string `json:"targetId,omitempty"`
	ItemID    string `json:"itemId,omitempty"`
	Position  *Vec2  `json:"position,omitempty"`
	Reeling   bool   `json:"reeling,omitempty"`
}
type PersonalState struct {
	Owner struct {
		Kind   string `json:"kind"`
		RoomID string `json:"roomId"`
		UserID string `json:"userId"`
	} `json:"owner"`
	Inventory map[string]int `json:"inventory"`
	Survival  SurvivalState  `json:"survival"`
}
type SurvivalState struct {
	Health    float64 `json:"health"`
	Hunger    float64 `json:"hunger"`
	Thirst    float64 `json:"thirst"`
	Energy    float64 `json:"energy"`
	UpdatedAt int64   `json:"updatedAt"`
}
type LifeState struct {
	Plots   map[string]PlotState      `json:"plots"`
	Animals map[string]AnimalState    `json:"animals"`
	Fishing map[string]FishingSession `json:"fishing"`
}
type PlotState struct {
	ID             string  `json:"id"`
	BuildingID     string  `json:"buildingId"`
	State          string  `json:"state"`
	CropID         string  `json:"cropId"`
	PlantedAt      int64   `json:"plantedAt"`
	ReadyAt        int64   `json:"readyAt"`
	WateredAt      int64   `json:"wateredAt"`
	GrowthModifier float64 `json:"growthModifier"`
}
type AnimalState struct {
	ID           string `json:"id"`
	DefinitionID string `json:"definitionId"`
	Position     Vec2   `json:"position"`
	Home         Vec2   `json:"home"`
	Target       Vec2   `json:"target"`
	State        string `json:"state"`
	FedAt        int64  `json:"fedAt"`
	NextEggAt    int64  `json:"nextEggAt"`
	EggReady     bool   `json:"eggReady"`
	EggPosition  *Vec2  `json:"eggPosition,omitempty"`
	UpdatedAt    int64  `json:"updatedAt"`
}
type FishingSession struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	SpotID    string  `json:"spotId"`
	FishID    string  `json:"fishId"`
	State     string  `json:"state"`
	Position  Vec2    `json:"position"`
	Origin    Vec2    `json:"origin"`
	StartedAt int64   `json:"startedAt"`
	BiteAt    int64   `json:"biteAt"`
	ExpiresAt int64   `json:"expiresAt"`
	Distance  float64 `json:"distance"`
	Tension   float64 `json:"tension"`
	Reeling   bool    `json:"reeling"`
	UpdatedAt int64   `json:"updatedAt"`
}
type LifeReceipt struct{ UserID, Payload string }

func initializeLife(f *FreeTowerState, at int64) {
	if f.Life == nil {
		f.Life = &LifeState{map[string]PlotState{}, map[string]AnimalState{}, map[string]FishingSession{}}
		for i := 0; i < lifeConfig.Animals.Count; i++ {
			p := Vec2{lifeConfig.Animals.Center.X + float64(i) - 1.5, lifeConfig.Animals.Center.Z + 3}
			id := fmt.Sprintf("chicken-%d", i+1)
			f.Life.Animals[id] = AnimalState{ID: id, DefinitionID: "chicken", Position: p, Home: p, Target: p, State: "idle", UpdatedAt: at}
		}
	}
	if f.Personal == nil {
		f.Personal = map[string]PersonalState{}
	}
	if f.LifeReceipts == nil {
		f.LifeReceipts = map[string]LifeReceipt{}
	}
	for id, b := range f.Buildings {
		if b.DefinitionID == "farm-plot" && b.Status == "completed" {
			if _, ok := f.Life.Plots[id]; !ok {
				f.Life.Plots[id] = PlotState{ID: id, BuildingID: id, State: "empty", GrowthModifier: 1}
			}
		}
	}
}
func (rr *RuntimeRoom) saveLife(next FarmState, at int64, save func(RoomRecord) error) error {
	next.Version++
	next.FreeTower.Version++
	next.UpdatedAt = at
	r := rr.record
	r.State = next
	if e := save(r); e != nil {
		return gatherErr("persistence", "Unable to save life action; retry the same request")
	}
	rr.record.State = next
	rr.lastPersist = at
	rr.broadcast(map[string]any{"type": "state", "state": publicState(next), "serverTime": at})
	rr.sendLifeSelf(at)
	return nil
}
func (rr *RuntimeRoom) sendLifeSelfTo(c *Client, at int64) {
	if rr.record.State.FreeTower == nil || c.player.World != "freetower" {
		return
	}
	if p, ok := rr.record.State.FreeTower.Personal[c.user.ID]; ok {
		b, _ := json.Marshal(map[string]any{"type": "life-self", "personal": p, "serverTime": at})
		select {
		case c.send <- b:
		default:
			c.close()
		}
	}
}
func (rr *RuntimeRoom) sendLifeSelf(at int64) {
	for c := range rr.clients {
		rr.sendLifeSelfTo(c, at)
	}
}
func (rr *RuntimeRoom) replayLifeSelf(c *Client, at int64) {
	if last, ok := rr.lifeReplaySync[c]; ok && at-last < 200 {
		return
	}
	if rr.lifeReplaySync == nil {
		rr.lifeReplaySync = map[*Client]int64{}
	}
	rr.lifeReplaySync[c] = at
	rr.sendLifeSelfTo(c, at)
}

func (rr *RuntimeRoom) joinLife(c *Client, at int64, save func(RoomRecord) error) error {
	rr.ensureGathering()
	next := cloneState(rr.record.State)
	initializeLife(next.FreeTower, at)
	if _, ok := next.FreeTower.Personal[c.user.ID]; !ok {
		p := PersonalState{Inventory: map[string]int{}, Survival: SurvivalState{100, 100, 100, 100, at}}
		p.Owner.Kind = "player"
		p.Owner.RoomID = rr.record.Room.ID
		p.Owner.UserID = c.user.ID
		for id, n := range lifeConfig.StarterItems {
			p.Inventory[id] = n
		}
		next.FreeTower.Personal[c.user.ID] = p
	}
	if f, ok := next.FreeTower.Life.Fishing[c.user.ID]; ok && liveFishing(f) {
		f.State = "cancelled"
		f.Reeling = false
		f.UpdatedAt = at
		next.FreeTower.Life.Fishing[c.user.ID] = f
	}
	return rr.saveLife(next, at, save)
}
func activeTool(rr *RuntimeRoom, c *Client) string {
	h := rr.gatheringUser(c).Hands
	if h.Active == "left" {
		return h.Left
	}
	return h.Right
}
func liveFishing(f FishingSession) bool {
	return f.ID != "" && f.State != "caught" && f.State != "escaped" && f.State != "cancelled"
}
func distance(a, b Vec2) float64 { return math.Hypot(a.X-b.X, a.Z-b.Z) }
func (rr *RuntimeRoom) lifeCommand(c *Client, q LifeRequest, at int64, save func(RoomRecord) error) error {
	if e := rr.gatheringParticipant(c); e != nil {
		return e
	}
	if q.RequestID == "" || len(q.RequestID) > 96 {
		return gatherErr("request", "Invalid request ID")
	}
	if rr.record.State.FreeTower == nil || rr.record.State.FreeTower.Personal == nil {
		return gatherErr("personal", "Character is not ready")
	}
	raw, _ := json.Marshal(q)
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	f := rr.record.State.FreeTower
	if r, ok := f.LifeReceipts[q.RequestID]; ok {
		if r.UserID != c.user.ID || r.Payload != hash {
			return gatherErr("request-conflict", "Request ID already used")
		}
		rr.replayLifeSelf(c, at)
		return nil
	}
	// Durable replay is resolved before throttling. Repeated identical reel
	// intent is acknowledged without clone/save/broadcast or permanent receipt.
	if r, ok := rr.lifeNoopReceipts[q.RequestID]; ok {
		if r.UserID != c.user.ID || r.Payload != hash {
			return gatherErr("request-conflict", "Request ID already used")
		}
		return nil
	}
	if q.Action == "reel" && f.Life != nil {
		s := f.Life.Fishing[c.user.ID]
		if liveFishing(s) && s.Reeling == q.Reeling {
			if rr.lifeNoopReceipts == nil {
				rr.lifeNoopReceipts = map[string]LifeReceipt{}
			}
			rr.lifeNoopReceipts[q.RequestID] = LifeReceipt{c.user.ID, hash}
			rr.lifeNoopOrder = append(rr.lifeNoopOrder, q.RequestID)
			if len(rr.lifeNoopOrder) > 128 {
				delete(rr.lifeNoopReceipts, rr.lifeNoopOrder[0])
				rr.lifeNoopOrder = rr.lifeNoopOrder[1:]
			}
			return nil
		}
	}
	// Release/cancel are safety inputs: never keep tension rising merely
	// because another successful mutation just consumed the rate budget.
	safety := q.Action == "cancel-fishing" || (q.Action == "reel" && !q.Reeling)
	if last, ok := rr.lifeLast[c.user.ID]; ok && !safety && at-last < 250 {
		return gatherErr("rate-limit", "Life action cooling down")
	}
	next := cloneState(rr.record.State)
	initializeLife(next.FreeTower, at)
	f = next.FreeTower
	p := f.Personal[c.user.ID]
	consume := func(id string) error {
		if p.Inventory[id] <= 0 {
			return gatherErr("inventory", "Item unavailable")
		}
		p.Inventory[id]--
		return nil
	}
	mode, _ := freeTowerLocomotion(c.player.Position)
	if mode == "swimming" && q.Action != "cancel-fishing" {
		return gatherErr("swimming", "Cannot interact while swimming")
	}
	if rr.gatheringUser(c).Action != nil {
		return gatherErr("busy", "Finish gathering first")
	}
	for _, b := range f.Buildings {
		if b.MoveOwnerID == c.user.ID {
			return gatherErr("busy", "Finish moving the building first")
		}
	}
	if liveFishing(f.Life.Fishing[c.user.ID]) && q.Action != "reel" && q.Action != "cancel-fishing" {
		return gatherErr("busy", "Finish fishing first")
	}
	var e error
	switch q.Action {
	case "cast":
		e = rr.castLife(c, q, f, &p, at)
	case "reel", "cancel-fishing":
		s := f.Life.Fishing[c.user.ID]
		if !liveFishing(s) {
			return gatherErr("fishing", "No active fishing session")
		}
		if q.Action == "cancel-fishing" {
			s.State = "cancelled"
			s.UpdatedAt = at
			s.Reeling = false
		} else {
			s.Reeling = q.Reeling
		}
		f.Life.Fishing[c.user.ID] = s
	case "till", "plant", "water", "harvest":
		e = rr.farmLife(c, q, f, &p, at)
	case "feed", "collect-egg":
		e = rr.animalLife(c, q, f, &p, at)
	case "eat":
		gain := map[string]float64{"carrot": 18, "wheat": 12, "egg": 22, "river-minnow": 12, "river-carp": 18, "coast-mackerel": 16, "coast-snapper": 20}[q.ItemID]
		if gain == 0 {
			return gatherErr("item", "Item cannot be eaten")
		}
		e = consume(q.ItemID)
		if e == nil {
			p.Survival.Hunger = math.Min(100, p.Survival.Hunger+gain)
			p.Survival.Energy = math.Min(100, p.Survival.Energy+8)
		}
	case "drink":
		if q.ItemID != "water-flask" {
			return gatherErr("item", "Use a water flask")
		}
		e = consume("water-flask")
		if e == nil {
			p.Survival.Thirst = math.Min(100, p.Survival.Thirst+25)
		}
	default:
		return gatherErr("action", "Unknown life action")
	}
	if e != nil {
		return e
	}
	f.Personal[c.user.ID] = p
	f.LifeReceipts[q.RequestID] = LifeReceipt{c.user.ID, hash}
	if e := rr.saveLife(next, at, save); e != nil {
		return e
	}
	if rr.lifeLast == nil {
		rr.lifeLast = map[string]int64{}
	}
	rr.lifeLast[c.user.ID] = at
	return nil
}

// Suppress repeated successful replay acknowledgements per socket, while
// retaining prompt replies for new mutations and safety inputs.
func (rr *RuntimeRoom) lifeReplayReplyAllowed(c *Client, at int64) bool {
	if last, ok := rr.lifeReplayReply[c]; ok && at-last < 200 {
		return false
	}
	if rr.lifeReplayReply == nil {
		rr.lifeReplayReply = map[*Client]int64{}
	}
	rr.lifeReplayReply[c] = at
	return true
}
