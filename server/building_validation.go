package main

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:embed freetower_buildings.json
var buildingJSON []byte

type BuildingDefinition struct {
	ID        string                         `json:"id"`
	Footprint struct{ Width, Depth float64 } `json:"footprint"`
	Cost      []struct {
		ItemID   string `json:"itemId"`
		Quantity int    `json:"quantity"`
	} `json:"cost"`
	ConstructionSeconds int64                                                                              `json:"constructionSeconds"`
	RotationStep        float64                                                                            `json:"rotationStep"`
	PlacementRules      struct{ MaxSlope, MaxHeightDelta, MinElevation, RiverBuffer, CoastBuffer float64 } `json:"placementRules"`
}

var buildingConfig = func() struct {
	PlacementRange, PlayerRadius float64
	RateLimitMs, MoveLockMs      int64
	ProtectedZones               []struct {
		ID           string
		X, Z, Radius float64
	}
	Definitions []BuildingDefinition
} {
	var c struct {
		PlacementRange, PlayerRadius float64
		RateLimitMs, MoveLockMs      int64
		ProtectedZones               []struct {
			ID           string
			X, Z, Radius float64
		}
		Definitions []BuildingDefinition
	}
	if e := json.Unmarshal(buildingJSON, &c); e != nil {
		panic(e)
	}
	return c
}()

func buildingDefinition(id string) *BuildingDefinition {
	for i := range buildingConfig.Definitions {
		if buildingConfig.Definitions[i].ID == id {
			return &buildingConfig.Definitions[i]
		}
	}
	return nil
}
func buildingLocal(p, center Vec2, a float64) Vec2 {
	dx, dz := p.X-center.X, p.Z-center.Z
	return Vec2{dx*math.Cos(a) - dz*math.Sin(a), dx*math.Sin(a) + dz*math.Cos(a)}
}
func buildingPoint(p Vec2, a, x, z float64) Vec2 {
	return Vec2{p.X + x*math.Cos(a) + z*math.Sin(a), p.Z - x*math.Sin(a) + z*math.Cos(a)}
}
func buildingCircle(p Vec2, a float64, d *BuildingDefinition, q Vec2, r float64) bool {
	v := buildingLocal(q, p, a)
	dx := math.Max(0, math.Abs(v.X)-d.Footprint.Width/2)
	dz := math.Max(0, math.Abs(v.Z)-d.Footprint.Depth/2)
	return dx*dx+dz*dz <= r*r
}
func buildingOverlap(p Vec2, a float64, d *BuildingDefinition, b BuildingInstance) bool {
	e := buildingDefinition(b.DefinitionID)
	if e == nil {
		return false
	}
	q := Vec2{b.Position.X, b.Position.Z}
	for _, ang := range []float64{a, a + math.Pi/2, b.RotationY, b.RotationY + math.Pi/2} {
		ux, uz := math.Cos(ang), -math.Sin(ang)
		radius := func(dd *BuildingDefinition, aa float64) float64 {
			return math.Abs(ux*math.Cos(aa)-uz*math.Sin(aa))*dd.Footprint.Width/2 + math.Abs(ux*math.Sin(aa)+uz*math.Cos(aa))*dd.Footprint.Depth/2
		}
		if math.Abs((p.X-q.X)*ux+(p.Z-q.Z)*uz) >= radius(d, a)+radius(e, b.RotationY)+.15 {
			return false
		}
	}
	return true
}

type buildingTree struct {
	p Vec2
	s float64
}

var buildingTrees = func() []buildingTree {
	out := []buildingTree{}
	add := func(x, z, s float64) {
		if math.Abs(z+19+math.Sin(x*.08)*5) > 2.8 && math.Abs(z-(freeTowerConfig.River.Center+math.Sin(x*freeTowerConfig.River.Frequency)*freeTowerConfig.River.Amplitude)) > 7 {
			out = append(out, buildingTree{Vec2{x, z}, s})
		}
	}
	for i := 0; i < 170; i++ {
		r := math.Sqrt((float64(i)+.5)/170) * 27
		a := float64(i) * 2.399963
		add(-37+math.Cos(a)*r, -23+math.Sin(a)*r, .85+(math.Sin(float64(i)*7.31)+1)*.45)
	}
	for i := 0; i < 26; i++ {
		sign := 1.
		if i%2 == 0 {
			sign = -1
		}
		add(sign*(16+float64(i%7)*3.7), 17+float64(i)/26*16, .8+float64(i%4)*.16)
	}
	return out
}()

func (rr *RuntimeRoom) validateBuilding(c *Client, d *BuildingDefinition, p Vec2, a float64, ignore string, cost bool) error {
	if d == nil {
		return gatherErr("definition", "Unknown building")
	}
	if math.IsNaN(p.X) || math.IsNaN(p.Z) || math.IsInf(p.X, 0) || math.IsInf(p.Z, 0) || math.IsNaN(a) || math.IsInf(a, 0) {
		return gatherErr("invalid", "Invalid coordinates")
	}
	step := d.RotationStep * math.Pi / 180
	if math.Abs(a/step-math.Round(a/step)) > 1e-6 {
		return gatherErr("rotation", "Invalid rotation step")
	}
	for _, z := range buildingConfig.ProtectedZones {
		if buildingCircle(p, a, d, Vec2{z.X, z.Z}, z.Radius) {
			return gatherErr("protected", "Protected landmark")
		}
	}
	nx, nz := int(math.Ceil(d.Footprint.Width)), int(math.Ceil(d.Footprint.Depth))
	samples := []Vec2{p}
	for i := 0; i <= nx; i++ {
		for j := 0; j <= nz; j++ {
			samples = append(samples, buildingPoint(p, a, -d.Footprint.Width/2+float64(i)*d.Footprint.Width/float64(nx), -d.Footprint.Depth/2+float64(j)*d.Footprint.Depth/float64(nz)))
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, q := range samples {
		if math.Abs(q.X) > freeTowerConfig.Bound || math.Abs(q.Z) > freeTowerConfig.Bound {
			return gatherErr("bounds", "Outside world")
		}
		h := freeTowerHeight(q)
		r := freeTowerConfig.River
		if math.Abs(q.Z-(r.Center+math.Sin(q.X*r.Frequency)*r.Amplitude)) < r.HalfWidth+d.PlacementRules.RiverBuffer || q.Z >= freeTowerConfig.Ocean.Shore+freeTowerCoastOffset(q.X)-d.PlacementRules.CoastBuffer || h < d.PlacementRules.MinElevation {
			return gatherErr("water", "Keep clear of water")
		}
		lo = math.Min(lo, h)
		hi = math.Max(hi, h)
		sx := (freeTowerHeight(Vec2{q.X + .25, q.Z}) - freeTowerHeight(Vec2{q.X - .25, q.Z})) / .5
		sz := (freeTowerHeight(Vec2{q.X, q.Z + .25}) - freeTowerHeight(Vec2{q.X, q.Z - .25})) / .5
		if math.Hypot(sx, sz) > d.PlacementRules.MaxSlope {
			return gatherErr("slope", "Ground too steep")
		}
	}
	if hi-lo > d.PlacementRules.MaxHeightDelta {
		return gatherErr("height-delta", "Ground uneven")
	}
	if math.Hypot(p.X-c.player.Position.X, p.Z-c.player.Position.Z) > buildingConfig.PlacementRange {
		return gatherErr("range", "Move closer")
	}
	for _, n := range rr.record.State.FreeTower.Nodes {
		scale := n.Scale
		// Legacy saved nodes predate scale; placement size comes from shared config.
		for _, canonical := range gatheringConfig.Nodes {
			if canonical.ID == n.ID {
				scale = canonical.Scale
				break
			}
		}
		r := .48*scale + .4
		if resourceDefinition(n.DefinitionID).Type == "rock" {
			r = 1.2*scale + .3
		}
		if buildingCircle(p, a, d, n.Position, r) {
			return gatherErr("resource", "Resource footprint reserved")
		}
	}
	for _, t := range buildingTrees {
		if buildingCircle(p, a, d, t.p, .48*t.s+.3) {
			return gatherErr("tree", "Tree footprint reserved")
		}
	}
	for id, b := range rr.record.State.FreeTower.Buildings {
		if id != ignore && buildingOverlap(p, a, d, b) {
			return gatherErr("overlap", "Building overlap")
		}
	}
	for other := range rr.clients {
		if other.player.World == "freetower" && buildingCircle(p, a, d, other.player.Position, buildingConfig.PlayerRadius) {
			return gatherErr("player", "Player in footprint")
		}
	}
	if cost {
		for _, v := range d.Cost {
			if rr.record.State.FreeTower.Inventory[v.ItemID] < v.Quantity {
				return gatherErr("inventory", "Insufficient resources")
			}
		}
	}
	return nil
}
func buildingMovement(f *FreeTowerState, old, p Vec2) Vec2 {
	if f == nil {
		return p
	}
	steps := int(math.Ceil(math.Hypot(p.X-old.X, p.Z-old.Z) / .2))
	if steps < 1 {
		steps = 1
	}
	for i := 1; i <= steps; i++ {
		q := Vec2{old.X + (p.X-old.X)*float64(i)/float64(steps), old.Z + (p.Z-old.Z)*float64(i)/float64(steps)}
		for _, b := range f.Buildings {
			d := buildingDefinition(b.DefinitionID)
			if d != nil && (b.Status == "completed" || b.Status == "moving") && buildingCircle(Vec2{b.Position.X, b.Position.Z}, b.RotationY, d, q, buildingConfig.PlayerRadius) {
				previous := Vec2{old.X + (p.X-old.X)*float64(i-1)/float64(steps), old.Z + (p.Z-old.Z)*float64(i-1)/float64(steps)}
				center := Vec2{b.Position.X, b.Position.Z}
				if buildingCircle(center, b.RotationY, d, previous, buildingConfig.PlayerRadius) {
					depth := func(point Vec2) float64 {
						v := buildingLocal(point, center, b.RotationY)
						return math.Min(d.Footprint.Width/2+buildingConfig.PlayerRadius-math.Abs(v.X), d.Footprint.Depth/2+buildingConfig.PlayerRadius-math.Abs(v.Z))
					}
					if depth(q) < depth(previous)-1e-9 {
						continue
					}
				}
				return old
			}
		}
	}
	return p
}
