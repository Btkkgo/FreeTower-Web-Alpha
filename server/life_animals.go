package main

import (
	"math"
	"reflect"
)

func (rr *RuntimeRoom) animalLife(c *Client, q LifeRequest, f *FreeTowerState, p *PersonalState, at int64) error {
	a, ok := f.Life.Animals[q.TargetID]
	if !ok {
		return gatherErr("animal", "Unknown animal")
	}
	target := a.Position
	if q.Action == "collect-egg" && a.EggPosition != nil {
		target = *a.EggPosition
	}
	if distance(c.player.Position, target) > lifeConfig.InteractionRange {
		return gatherErr("range", "Move closer")
	}
	if q.Action == "feed" {
		if a.NextEggAt > 0 || a.EggReady {
			return gatherErr("egg-pending", "Collect the egg first")
		}
		if q.ItemID != "chicken-feed" || p.Inventory["chicken-feed"] <= 0 {
			return gatherErr("inventory", "No chicken feed")
		}
		p.Inventory["chicken-feed"]--
		a.FedAt = at
		a.NextEggAt = at + lifeConfig.Animals.EggSeconds*1000
		a.State = "approach-food"
		a.Target = c.player.Position
	} else {
		if !a.EggReady {
			return gatherErr("egg", "No egg ready")
		}
		p.Inventory["egg"]++
		a.EggReady = false
		a.EggPosition = nil
		a.NextEggAt = 0
	}
	f.Life.Animals[a.ID] = a
	return nil
}
func animalAllowed(f *FreeTowerState, old, p Vec2) bool {
	mode, _ := freeTowerLocomotion(p)
	return mode == "land" && buildingMovement(f, old, p) == p
}
func (rr *RuntimeRoom) advanceLife(at int64, restart bool, disconnected *Client, save func(RoomRecord) error) error {
	if rr.record.State.FreeTower == nil {
		return nil
	}
	next := cloneState(rr.record.State)
	f := next.FreeTower
	initializeLife(f, at)
	for id, p := range f.Personal {
		dt := math.Max(0, float64(at-p.Survival.UpdatedAt)/60000)
		p.Survival.Hunger = math.Max(0, p.Survival.Hunger-lifeConfig.Survival.HungerDecay*dt)
		p.Survival.Thirst = math.Max(0, p.Survival.Thirst-lifeConfig.Survival.ThirstDecay*dt)
		p.Survival.Energy = math.Min(100, p.Survival.Energy+lifeConfig.Survival.EnergyRegeneration*dt)
		p.Survival.UpdatedAt = at
		f.Personal[id] = p
	}
	for id, p := range f.Life.Plots {
		if p.WateredAt > 0 {
			if at >= p.ReadyAt {
				p.State = "ready"
			} else if at > p.WateredAt+1000 {
				p.State = "growing"
			}
		}
		f.Life.Plots[id] = p
	}
	for id, a := range f.Life.Animals {
		dt := math.Min(2, math.Max(0, float64(at-a.UpdatedAt)/1000))
		if !a.EggReady && a.NextEggAt > 0 && at >= a.NextEggAt {
			a.EggReady = true
			p := a.Position
			a.EggPosition = &p
		}
		if a.FedAt > 0 && at-a.FedAt < 2500 {
			a.State = "eat"
		} else {
			if distance(a.Position, a.Target) < .2 || !animalAllowed(f, a.Position, a.Target) {
				angle := lifeRandom() * 2 * math.Pi
				radius := lifeRandom() * lifeConfig.Animals.HabitatRadius
				a.Target = Vec2{lifeConfig.Animals.Center.X + math.Cos(angle)*radius, lifeConfig.Animals.Center.Z + math.Sin(angle)*radius}
				a.State = "peck"
			}
			d := distance(a.Position, a.Target)
			if d > .2 {
				step := math.Min(d, dt*lifeConfig.Animals.Speed)
				p := Vec2{a.Position.X + (a.Target.X-a.Position.X)/d*step, a.Position.Z + (a.Target.Z-a.Position.Z)/d*step}
				if animalAllowed(f, a.Position, p) {
					a.Position = p
					a.State = "wander"
				} else {
					a.Target = a.Position
					a.State = "idle"
				}
			}
		}
		a.UpdatedAt = at
		f.Life.Animals[id] = a
	}
	advanceFishing(f, rr.clients, at, restart, disconnected)
	if reflect.DeepEqual(next.FreeTower, rr.record.State.FreeTower) {
		return nil
	}
	return rr.saveLife(next, at, save)
}
