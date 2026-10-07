package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
)

type BuildingPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type BuildingInstance struct {
	ID                    string           `json:"id"`
	DefinitionID          string           `json:"definitionId"`
	Position              BuildingPosition `json:"position"`
	RotationY             float64          `json:"rotationY"`
	Status                string           `json:"status"`
	ConstructionStartedAt int64            `json:"constructionStartedAt"`
	ConstructionEndsAt    int64            `json:"constructionEndsAt"`
	OwnerID               string           `json:"ownerId"`
	MoveOwnerID           string           `json:"moveOwnerId,omitempty"`
	MoveExpiresAt         int64            `json:"moveExpiresAt,omitempty"`
}
type BuildingRequest struct {
	RequestID    string  `json:"requestId"`
	DefinitionID string  `json:"definitionId"`
	BuildingID   string  `json:"buildingId"`
	MoveToken    string  `json:"moveToken"`
	Position     Vec2    `json:"position"`
	RotationY    float64 `json:"rotationY"`
}
type BuildingResult struct {
	RequestID  string `json:"requestId"`
	BuildingID string `json:"buildingId"`
	MoveToken  string `json:"moveToken,omitempty"`
}
type BuildingReceipt struct {
	UserID, Payload string
	Result          BuildingResult
}
type buildingLock struct {
	Token  string
	Client *Client
}

func (rr *RuntimeRoom) saveBuildings(next FarmState, at int64, save func(RoomRecord) error) error {
	next.FreeTower.Version++
	next.Version++
	next.UpdatedAt = at
	r := rr.record
	r.State = next
	if e := save(r); e != nil {
		return gatherErr("persistence", "Unable to save building; try again")
	}
	rr.record.State = next
	rr.lastPersist = at
	rr.broadcast(map[string]any{"type": "state", "state": publicState(next), "serverTime": at})
	return nil
}
func (rr *RuntimeRoom) buildingCommand(c *Client, kind string, q BuildingRequest, at int64, save func(RoomRecord) error) (BuildingResult, error) {
	result := BuildingResult{RequestID: q.RequestID, BuildingID: q.BuildingID}
	if e := rr.gatheringParticipant(c); e != nil {
		return result, e
	}
	rr.ensureGathering()
	if rr.buildingLocks == nil {
		rr.buildingLocks = map[string]buildingLock{}
	}
	f := rr.record.State.FreeTower
	b := f.Buildings[q.BuildingID]
	durable := kind == "place-building" || kind == "move-building" || kind == "begin-building-move"
	raw, _ := json.Marshal(struct {
		Kind    string
		Request BuildingRequest
	}{kind, q})
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	if durable {
		if len(q.RequestID) < 1 || len(q.RequestID) > 96 {
			return result, gatherErr("request", "Invalid request ID")
		}
		if receipt, ok := f.BuildingReceipts[q.RequestID]; ok {
			if receipt.UserID != c.user.ID || receipt.Payload != hash {
				return result, gatherErr("request-conflict", "Request ID already used")
			}
			if kind == "begin-building-move" {
				lock, ok := rr.buildingLocks[receipt.Result.BuildingID]
				bb := f.Buildings[receipt.Result.BuildingID]
				if !ok || lock.Client != c || bb.MoveExpiresAt <= at {
					return result, gatherErr("lock-expired", "Move lock no longer active")
				}
			}
			return receipt.Result, nil
		}
	}
	if kind == "place-building" || kind == "begin-building-move" || kind == "move-building" {
		mode, _ := freeTowerLocomotion(c.player.Position)
		if mode != "land" || (c.player.MovementMode != "" && c.player.MovementMode != "land") {
			if kind == "move-building" {
				lock, ok := rr.buildingLocks[b.ID]
				if ok && lock.Client == c && lock.Token == q.MoveToken && b.MoveExpiresAt > at {
					if _, e := rr.buildingCommand(c, "cancel-building-move", BuildingRequest{BuildingID: b.ID, MoveToken: q.MoveToken}, at, save); e != nil {
						return result, e
					}
				}
			}
			return result, gatherErr("swimming", "Cannot build while swimming")
		}
		if rr.buildingLast != nil && at-rr.buildingLast[c.user.ID] < buildingConfig.RateLimitMs {
			return result, gatherErr("rate-limit", "Building action cooling down")
		}
	}
	next := cloneState(rr.record.State)
	if next.FreeTower.Buildings == nil {
		next.FreeTower.Buildings = map[string]BuildingInstance{}
	}
	if next.FreeTower.BuildingReceipts == nil {
		next.FreeTower.BuildingReceipts = map[string]BuildingReceipt{}
	}
	var token string
	switch kind {
	case "place-building":
		d := buildingDefinition(q.DefinitionID)
		if e := rr.validateBuilding(c, d, q.Position, q.RotationY, "", true); e != nil {
			return result, e
		}
		b = BuildingInstance{ID: newID(), DefinitionID: d.ID, Position: BuildingPosition{q.Position.X, freeTowerHeight(q.Position), q.Position.Z}, RotationY: q.RotationY, Status: "constructing", ConstructionStartedAt: at, ConstructionEndsAt: at + d.ConstructionSeconds*1000, OwnerID: c.user.ID}
		for _, v := range d.Cost {
			next.FreeTower.Inventory[v.ItemID] -= v.Quantity
		}
		result.BuildingID = b.ID
	case "begin-building-move":
		if b.ID == "" {
			return result, gatherErr("building", "Unknown building")
		}
		if b.Status != "completed" {
			return result, gatherErr("locked", "Building unavailable")
		}
		if math.Hypot(c.player.Position.X-b.Position.X, c.player.Position.Z-b.Position.Z) > buildingConfig.PlacementRange {
			return result, gatherErr("range", "Move closer")
		}
		token = newID()
		result.MoveToken = token
		b.Status = "moving"
		b.MoveOwnerID = c.user.ID
		b.MoveExpiresAt = at + buildingConfig.MoveLockMs
	case "move-building", "cancel-building-move", "renew-building-move":
		lock, ok := rr.buildingLocks[b.ID]
		if !ok || b.Status != "moving" || b.MoveOwnerID != c.user.ID || lock.Client != c || lock.Token != q.MoveToken || at >= b.MoveExpiresAt {
			return result, gatherErr("lock", "Invalid or expired move token")
		}
		if kind == "renew-building-move" {
			b.MoveExpiresAt = at + buildingConfig.MoveLockMs
		} else {
			if kind == "move-building" {
				if e := rr.validateBuilding(c, buildingDefinition(b.DefinitionID), q.Position, q.RotationY, b.ID, false); e != nil {
					_, cancelErr := rr.buildingCommand(c, "cancel-building-move", BuildingRequest{BuildingID: b.ID, MoveToken: q.MoveToken}, at, save)
					if cancelErr != nil {
						return result, cancelErr
					}
					return result, e
				}
				b.Position = BuildingPosition{q.Position.X, freeTowerHeight(q.Position), q.Position.Z}
				b.RotationY = q.RotationY
			}
			b.Status = "completed"
			b.MoveOwnerID = ""
			b.MoveExpiresAt = 0
		}
	default:
		return result, gatherErr("invalid", "Unknown building operation")
	}
	next.FreeTower.Buildings[b.ID] = b
	if durable {
		next.FreeTower.BuildingReceipts[q.RequestID] = BuildingReceipt{c.user.ID, hash, result}
	}
	if e := rr.saveBuildings(next, at, save); e != nil {
		return result, e
	}
	if kind == "begin-building-move" {
		rr.buildingLocks[b.ID] = buildingLock{token, c}
	} else if b.Status != "moving" {
		delete(rr.buildingLocks, b.ID)
	}
	if durable {
		if rr.buildingLast == nil {
			rr.buildingLast = map[string]int64{}
		}
		rr.buildingLast[c.user.ID] = at
	}
	rr.cancelGather(c)
	return result, nil
}
func (rr *RuntimeRoom) advanceBuildings(at int64, restart bool, disconnected *Client, save func(RoomRecord) error) error {
	if rr.record.State.FreeTower == nil {
		return nil
	}
	next := cloneState(rr.record.State)
	changed := false
	ids := []string{}
	for id, b := range next.FreeTower.Buildings {
		lock := rr.buildingLocks[id]
		if b.Status == "constructing" && at >= b.ConstructionEndsAt {
			b.Status = "completed"
			changed = true
		}
		if b.Status == "moving" && (restart || at >= b.MoveExpiresAt || (disconnected != nil && lock.Client == disconnected)) {
			b.Status = "completed"
			b.MoveOwnerID = ""
			b.MoveExpiresAt = 0
			changed = true
			ids = append(ids, id)
		}
		next.FreeTower.Buildings[id] = b
	}
	if !changed {
		return nil
	}
	if e := rr.saveBuildings(next, at, save); e != nil {
		return e
	}
	for _, id := range ids {
		delete(rr.buildingLocks, id)
	}
	return nil
}

// Persistent receipts are private authority data, never part of a client snapshot.
func publicState(s FarmState) FarmState {
	if s.FreeTower == nil {
		return s
	}
	f := *s.FreeTower
	f.BuildingReceipts = nil
	s.FreeTower = &f
	return s
}
