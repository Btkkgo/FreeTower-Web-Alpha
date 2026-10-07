package main

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:embed freetower_resources.json
var gatheringJSON []byte

type HandState struct {
	Left   string `json:"left"`
	Right  string `json:"right"`
	Active string `json:"active"`
}
type GatherAction struct {
	ID         string `json:"id"`
	ResourceID string `json:"resourceId"`
	ToolID     string `json:"toolId"`
	StartedAt  int64  `json:"startedAt"`
	HitAt      int64  `json:"hitAt"`
	EndsAt     int64  `json:"endsAt"`
}
type ResourceNodeState struct {
	Scale         float64 `json:"scale,omitempty"`
	ID            string  `json:"id"`
	DefinitionID  string  `json:"definitionId"`
	Position      Vec2    `json:"position"`
	Durability    int     `json:"durability"`
	MaxDurability int     `json:"maxDurability"`
	Depleted      bool    `json:"depleted"`
	RespawnAt     int64   `json:"respawnAt"`
}
type FreeTowerState struct {
	Life             *LifeState                   `json:"life,omitempty"`
	Personal         map[string]PersonalState     `json:"personal,omitempty"`
	LifeReceipts     map[string]LifeReceipt       `json:"lifeReceipts,omitempty"`
	Buildings        map[string]BuildingInstance  `json:"buildings,omitempty"`
	BuildingReceipts map[string]BuildingReceipt   `json:"buildingReceipts,omitempty"`
	Version          int                          `json:"version"`
	Nodes            map[string]ResourceNodeState `json:"nodes"`
	Inventory        map[string]int               `json:"inventory"`
}
type ToolDefinition struct {
	ID                 string   `json:"id"`
	Type               string   `json:"type"`
	GatherPower        int      `json:"gatherPower"`
	Cooldown           int64    `json:"cooldown"`
	ValidResourceTypes []string `json:"validResourceTypes"`
}
type ResourceDefinition struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	MaxDurability  int    `json:"maxDurability"`
	RequiredTool   string `json:"requiredTool"`
	RespawnSeconds int64  `json:"respawnSeconds"`
	Drops          []struct {
		ItemID   string `json:"itemId"`
		Quantity int    `json:"quantity"`
	} `json:"drops"`
}

var gatheringConfig = func() struct {
	InteractionRange float64              `json:"interactionRange"`
	WindupMS         int64                `json:"windupMs"`
	RecoveryMS       int64                `json:"recoveryMs"`
	Tools            []ToolDefinition     `json:"tools"`
	Definitions      []ResourceDefinition `json:"definitions"`
	Nodes            []ResourceNodeState  `json:"nodes"`
} {
	var cfg struct {
		InteractionRange float64              `json:"interactionRange"`
		WindupMS         int64                `json:"windupMs"`
		RecoveryMS       int64                `json:"recoveryMs"`
		Tools            []ToolDefinition     `json:"tools"`
		Definitions      []ResourceDefinition `json:"definitions"`
		Nodes            []ResourceNodeState  `json:"nodes"`
	}
	if err := json.Unmarshal(gatheringJSON, &cfg); err != nil {
		panic(err)
	}
	return cfg
}()

type gatheringUser struct {
	Hands         HandState
	Action        *GatherAction
	CooldownUntil int64
	Consumed      bool
	LastID        string
}
type gatherError struct {
	Code    string
	Message string
}

func (e *gatherError) Error() string   { return e.Message }
func gatherErr(code, msg string) error { return &gatherError{code, msg} }
func defaultHands() *HandState {
	return &HandState{Left: "basic-pickaxe", Right: "basic-axe", Active: "right"}
}
func resourceDefinition(id string) *ResourceDefinition {
	for i := range gatheringConfig.Definitions {
		if gatheringConfig.Definitions[i].ID == id {
			return &gatheringConfig.Definitions[i]
		}
	}
	return nil
}
func toolDefinition(id string) *ToolDefinition {
	for i := range gatheringConfig.Tools {
		if gatheringConfig.Tools[i].ID == id {
			return &gatheringConfig.Tools[i]
		}
	}
	return nil
}

// All room gathering operations run under rr.mu, including persistence and broadcasts.
func (rr *RuntimeRoom) ensureGathering() {
	if rr.record.State.FreeTower != nil {
		return
	}
	f := &FreeTowerState{Version: 1, Nodes: map[string]ResourceNodeState{}, Inventory: map[string]int{"wood": 0, "stone": 0}}
	for _, n := range gatheringConfig.Nodes {
		d := resourceDefinition(n.DefinitionID)
		n.MaxDurability = d.MaxDurability
		n.Durability = d.MaxDurability
		f.Nodes[n.ID] = n
	}
	rr.record.State.FreeTower = f
}
func (rr *RuntimeRoom) gatheringParticipant(c *Client) error {
	if rr.deleted || !rr.clients[c] || !rr.record.Members[c.user.ID] || c.player.ID != c.user.ID || c.player.World != "freetower" {
		return gatherErr("participant", "Join the FreeTower room first")
	}
	return nil
}
func (rr *RuntimeRoom) gatheringUser(c *Client) *gatheringUser {
	if rr.gatherUsers == nil {
		rr.gatherUsers = map[string]*gatheringUser{}
	}
	u := rr.gatherUsers[c.user.ID]
	if u == nil {
		u = &gatheringUser{Hands: *defaultHands()}
		rr.gatherUsers[c.user.ID] = u
	}
	return u
}
func (rr *RuntimeRoom) syncGatherUser(id string) {
	u := rr.gatherUsers[id]
	if u == nil {
		return
	}
	for c := range rr.clients {
		if c.user.ID == id && c.player.World == "freetower" {
			h := u.Hands
			c.player.Hands = &h
			if u.Action == nil {
				c.player.GatherAction = nil
			} else {
				a := *u.Action
				c.player.GatherAction = &a
			}
		}
	}
}
func (rr *RuntimeRoom) setHands(c *Client, h HandState) error {
	if err := rr.gatheringParticipant(c); err != nil {
		return err
	}
	rr.ensureGathering()
	if h.Active != "left" && h.Active != "right" {
		return gatherErr("hand", "Invalid active hand")
	}
	for _, id := range []string{h.Left, h.Right} {
		if id == "empty" || toolDefinition(id) != nil {
			continue
		}
		if personal, ok := rr.record.State.FreeTower.Personal[c.user.ID]; ok && personal.Inventory[id] > 0 {
			continue
		}
		if (id == "wood" || id == "stone") && rr.record.State.FreeTower.Inventory[id] > 0 {
			continue
		}
		return gatherErr("ownership", "Held item is unavailable")
	}
	u := rr.gatheringUser(c)
	u.Hands = h
	u.Action = nil
	rr.syncGatherUser(c.user.ID)
	rr.broadcastPresence()
	return nil
}
func (rr *RuntimeRoom) validateGather(c *Client, rid, tid string) (*ToolDefinition, ResourceNodeState, error) {
	if rr.record.State.FreeTower != nil && rr.record.State.FreeTower.Life != nil && liveFishing(rr.record.State.FreeTower.Life.Fishing[c.user.ID]) {
		return nil, ResourceNodeState{}, gatherErr("busy", "Finish fishing first")
	}
	if err := rr.gatheringParticipant(c); err != nil {
		return nil, ResourceNodeState{}, err
	}
	rr.ensureGathering()
	n, ok := rr.record.State.FreeTower.Nodes[rid]
	if !ok {
		return nil, n, gatherErr("resource", "Unknown resource")
	}
	if n.Depleted || n.Durability <= 0 {
		return nil, n, gatherErr("depleted", "Resource is depleted")
	}
	mode, _ := freeTowerLocomotion(c.player.Position)
	if mode == "swimming" || c.player.MovementMode == "swimming" {
		return nil, n, gatherErr("swimming", "Cannot gather while swimming")
	}
	if math.IsNaN(c.player.Position.X) || math.IsNaN(c.player.Position.Z) || math.Hypot(c.player.Position.X-n.Position.X, c.player.Position.Z-n.Position.Z) > gatheringConfig.InteractionRange {
		return nil, n, gatherErr("range", "Move closer to the resource")
	}
	tool := toolDefinition(tid)
	if tool == nil {
		return nil, n, gatherErr("ownership", "Tool is unavailable")
	}
	u := rr.gatheringUser(c)
	held := u.Hands.Right
	if u.Hands.Active == "left" {
		held = u.Hands.Left
	}
	if held != tid {
		return nil, n, gatherErr("hand", "Equip the tool in the active hand")
	}
	d := resourceDefinition(n.DefinitionID)
	valid := false
	for _, typ := range tool.ValidResourceTypes {
		if d != nil && typ == d.Type && tool.Type == d.RequiredTool {
			valid = true
		}
	}
	if !valid {
		return nil, n, gatherErr("wrong-tool", "Wrong tool")
	}
	return tool, n, nil
}
func (rr *RuntimeRoom) startGather(c *Client, rid, tid, id string, at int64) error {
	if _, _, err := rr.validateGather(c, rid, tid); err != nil {
		return err
	}
	u := rr.gatheringUser(c)
	if id == "" || len(id) > 96 || id == u.LastID {
		return gatherErr("action", "Invalid or reused action ID")
	}
	if at < u.CooldownUntil {
		return gatherErr("cooldown", "Tool is cooling down")
	}
	if u.Action != nil && at < u.Action.EndsAt {
		return gatherErr("busy", "Gather action is still active")
	}
	u.Action = &GatherAction{ID: id, ResourceID: rid, ToolID: tid, StartedAt: at, HitAt: at + gatheringConfig.WindupMS, EndsAt: at + gatheringConfig.WindupMS + gatheringConfig.RecoveryMS}
	u.Consumed = false
	rr.syncGatherUser(c.user.ID)
	rr.broadcastPresence()
	return nil
}
func (rr *RuntimeRoom) hitGather(c *Client, rid, tid, id string, at int64, save func(RoomRecord) error) error {
	tool, node, err := rr.validateGather(c, rid, tid)
	if err != nil {
		return err
	}
	u := rr.gatheringUser(c)
	a := u.Action
	if a == nil || u.Consumed || a.ID != id || a.ResourceID != rid || a.ToolID != tid {
		return gatherErr("action", "No matching gather action")
	}
	if at < a.HitAt {
		return gatherErr("windup", "Wait for the hit frame")
	}
	if at > a.EndsAt {
		return gatherErr("expired", "Gather action expired")
	}
	if at < u.CooldownUntil {
		return gatherErr("cooldown", "Tool is cooling down")
	}
	next := cloneState(rr.record.State)
	node.Durability -= tool.GatherPower
	if node.Durability <= 0 {
		node.Durability = 0
		node.Depleted = true
		d := resourceDefinition(node.DefinitionID)
		node.RespawnAt = at + d.RespawnSeconds*1000
		for _, drop := range d.Drops {
			next.FreeTower.Inventory[drop.ItemID] += drop.Quantity
		}
	}
	next.FreeTower.Nodes[rid] = node
	next.FreeTower.Version++
	next.Version++
	next.UpdatedAt = at
	record := rr.record
	record.State = next
	if err := save(record); err != nil {
		return gatherErr("persistence", "Unable to save resource hit; try again")
	}
	rr.record.State = next
	rr.lastPersist = at
	u.Consumed = true
	u.LastID = id
	u.CooldownUntil = a.StartedAt + tool.Cooldown
	rr.broadcast(map[string]any{"type": "state", "state": publicState(next), "serverTime": at})
	rr.broadcast(map[string]any{"type": "resource-hit", "resourceId": rid, "userId": c.user.ID, "actionId": id, "at": at, "depleted": node.Depleted})
	return nil
}
func (rr *RuntimeRoom) cancelGather(c *Client) {
	if rr.gatheringParticipant(c) != nil {
		return
	}
	u := rr.gatheringUser(c)
	u.Action = nil
	rr.syncGatherUser(c.user.ID)
}
func (rr *RuntimeRoom) expireGatherActions(at int64) bool {
	changed := false
	for id, u := range rr.gatherUsers {
		if u.Action != nil && at > u.Action.EndsAt {
			u.Action = nil
			rr.syncGatherUser(id)
			changed = true
		}
	}
	return changed
}

// Respawn uses durable absolute Unix milliseconds; a failed write leaves depleted nodes untouched.
func (rr *RuntimeRoom) respawnGathering(at int64, save func(RoomRecord) error) error {
	if rr.record.State.FreeTower == nil {
		return nil
	}
	next := cloneState(rr.record.State)
	changed := false
	for id, n := range next.FreeTower.Nodes {
		if n.Depleted && n.RespawnAt > 0 && at >= n.RespawnAt {
			n.Depleted = false
			n.Durability = n.MaxDurability
			n.RespawnAt = 0
			next.FreeTower.Nodes[id] = n
			changed = true
		}
	}
	if !changed {
		return nil
	}
	next.FreeTower.Version++
	next.Version++
	next.UpdatedAt = at
	record := rr.record
	record.State = next
	if err := save(record); err != nil {
		return err
	}
	rr.record.State = next
	rr.lastPersist = at
	return nil
}
