package main

import (
	"math"
	"math/rand"
)

var lifeRandom = rand.Float64

func (rr *RuntimeRoom) castLife(c *Client, q LifeRequest, f *FreeTowerState, p *PersonalState, at int64) error {
	if activeTool(rr, c) != "basic-fishing-rod" {
		return gatherErr("tool", "Equip fishing rod")
	}
	if q.Position == nil || math.IsNaN(q.Position.X) || math.IsNaN(q.Position.Z) || math.IsInf(q.Position.X, 0) || math.IsInf(q.Position.Z, 0) {
		return gatherErr("position", "Invalid cast position")
	}
	pos := *q.Position
	d := distance(c.player.Position, pos)
	if d < lifeConfig.Fishing.MinRange || d > lifeConfig.Fishing.MaxRange {
		return gatherErr("range", "Cast between 2 and 12 metres")
	}
	angle := math.Atan2(pos.X-c.player.Position.X, pos.Z-c.player.Position.Z)
	if math.Abs(math.Remainder(angle-c.player.Facing, 2*math.Pi)) > lifeConfig.Fishing.FacingConeDegrees*math.Pi/360 {
		return gatherErr("facing", "Face the water")
	}
	r := freeTowerConfig.River
	habitat := ""
	spot := ""
	if math.Abs(pos.Z-(r.Center+math.Sin(pos.X*r.Frequency)*r.Amplitude)) < r.HalfWidth {
		habitat = "river"
		spot = "silver-river"
	} else if pos.Z >= freeTowerConfig.Ocean.Shore+freeTowerCoastOffset(pos.X) {
		habitat = "coast"
		spot = "ancient-sea"
	}
	if habitat == "" || q.TargetID != spot {
		return gatherErr("water", "Cast into selected fishing spot")
	}
	for i := 1; i <= int(math.Ceil(d/.2)); i++ {
		t := float64(i) / math.Ceil(d/.2)
		v := Vec2{c.player.Position.X + (pos.X-c.player.Position.X)*t, c.player.Position.Z + (pos.Z-c.player.Position.Z)*t}
		for _, b := range f.Buildings {
			def := buildingDefinition(b.DefinitionID)
			if def != nil && b.DefinitionID != "farm-plot" && buildingCircle(Vec2{b.Position.X, b.Position.Z}, b.RotationY, def, v, .05) {
				return gatherErr("obstruction", "Building blocks the cast")
			}
		}
	}
	if p.Inventory["basic-bait"] <= 0 {
		return gatherErr("inventory", "No bait")
	}
	choices := []FishDefinition{}
	total := 0.0
	for _, fish := range lifeConfig.Fish {
		if fish.Habitat == habitat {
			choices = append(choices, fish)
			total += 1 / float64(fish.Rarity)
		}
	}
	pick := lifeRandom() * total
	fish := choices[len(choices)-1]
	for _, candidate := range choices {
		pick -= 1 / float64(candidate.Rarity)
		if pick <= 0 {
			fish = candidate
			break
		}
	}
	delay := fish.BiteDelayRange[0] + lifeRandom()*(fish.BiteDelayRange[1]-fish.BiteDelayRange[0])
	p.Inventory["basic-bait"]--
	f.Life.Fishing[c.user.ID] = FishingSession{ID: q.RequestID, UserID: c.user.ID, SpotID: spot, FishID: fish.ID, State: "casting", Position: pos, Origin: c.player.Position, StartedAt: at, BiteAt: at + int64(delay*1000), ExpiresAt: at + int64(lifeConfig.Fishing.ExpirySeconds*1000), Distance: lifeConfig.Fishing.InitialDistance, UpdatedAt: at}
	return nil
}
func advanceFishing(f *FreeTowerState, clients map[*Client]bool, at int64, restart bool, disconnected *Client) {
	for id, s := range f.Life.Fishing {
		if !liveFishing(s) {
			if at-s.UpdatedAt > 4000 {
				delete(f.Life.Fishing, id)
			}
			continue
		}
		var c *Client
		for other := range clients {
			if other.user.ID == id && other.player.World == "freetower" {
				c = other
				break
			}
		}
		invalid := restart || c == nil || (disconnected != nil && disconnected.user.ID == id)
		if c != nil {
			h := c.player.Hands
			held := ""
			if h != nil {
				held = h.Right
				if h.Active == "left" {
					held = h.Left
				}
			}
			mode, _ := freeTowerLocomotion(c.player.Position)
			invalid = invalid || held != "basic-fishing-rod" || mode == "swimming" || distance(c.player.Position, s.Origin) > 2
		}
		if invalid {
			s.State = "cancelled"
			s.Reeling = false
			s.UpdatedAt = at
			f.Life.Fishing[id] = s
			continue
		}
		if at >= s.ExpiresAt || (at >= s.BiteAt+8000 && s.Distance == lifeConfig.Fishing.InitialDistance) {
			s.State = "escaped"
		} else if at >= s.BiteAt {
			dt := float64(at-max(s.UpdatedAt, s.BiteAt)) / 1000
			if s.Reeling {
				s.State = "reeling"
				s.Distance = math.Max(0, s.Distance-lifeConfig.Fishing.ReelSpeed*dt)
				strength := 1.0
				for _, fish := range lifeConfig.Fish {
					if fish.ID == s.FishID {
						strength = fish.FightStrength
					}
				}
				s.Tension += lifeConfig.Fishing.TensionGain * strength * dt
			} else {
				s.State = "hooked"
				s.Tension = math.Max(0, s.Tension-lifeConfig.Fishing.TensionRelax*dt)
			}
			if s.Tension >= lifeConfig.Fishing.MaxTension {
				s.State = "escaped"
			} else if s.Distance <= 0 {
				s.State = "caught"
				p := f.Personal[id]
				p.Inventory[s.FishID]++
				f.Personal[id] = p
			}
		} else if at >= s.BiteAt-700 {
			s.State = "nibbling"
		} else if at >= s.BiteAt-1600 {
			s.State = "fish-noticed"
		} else if at > s.StartedAt+500 {
			s.State = "waiting"
		}
		s.UpdatedAt = at
		f.Life.Fishing[id] = s
	}
}
