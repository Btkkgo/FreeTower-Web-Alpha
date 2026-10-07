package main

func (rr *RuntimeRoom) farmLife(c *Client, q LifeRequest, f *FreeTowerState, p *PersonalState, at int64) error {
	plot, ok := f.Life.Plots[q.TargetID]
	b := f.Buildings[plot.BuildingID]
	if !ok || b.Status != "completed" {
		return gatherErr("plot", "Plot unavailable")
	}
	if distance(c.player.Position, Vec2{b.Position.X, b.Position.Z}) > lifeConfig.InteractionRange {
		return gatherErr("range", "Move closer to plot")
	}
	switch q.Action {
	case "till":
		if activeTool(rr, c) != "basic-hoe" {
			return gatherErr("tool", "Equip a hoe")
		}
		if plot.State != "empty" {
			return gatherErr("plot-state", "Plot already tilled")
		}
		plot.State = "tilled"
	case "plant":
		if plot.State != "tilled" {
			return gatherErr("plot-state", "Till empty soil first")
		}
		var crop *CropDefinition
		for i := range lifeConfig.Crops {
			if lifeConfig.Crops[i].SeedID == q.ItemID {
				crop = &lifeConfig.Crops[i]
			}
		}
		if crop == nil {
			return gatherErr("seed", "Unknown seed")
		}
		if p.Inventory[q.ItemID] <= 0 {
			return gatherErr("inventory", "No seed")
		}
		p.Inventory[q.ItemID]--
		plot.CropID = crop.ID
		plot.PlantedAt = at
		plot.State = "planted"
	case "water":
		if activeTool(rr, c) != "basic-watering-can" {
			return gatherErr("tool", "Equip watering can")
		}
		if plot.State != "planted" {
			return gatherErr("plot-state", "Plant a seed first")
		}
		for _, crop := range lifeConfig.Crops {
			if crop.ID == plot.CropID {
				plot.ReadyAt = at + crop.GrowthSeconds*1000
			}
		}
		plot.WateredAt = at
		plot.State = "watered"
	case "harvest":
		if plot.ReadyAt == 0 || at < plot.ReadyAt {
			return gatherErr("plot-state", "Crop is not ready")
		}
		for _, crop := range lifeConfig.Crops {
			if crop.ID == plot.CropID {
				p.Inventory[crop.ID] += crop.HarvestQuantity
			}
		}
		plot = PlotState{ID: plot.ID, BuildingID: plot.BuildingID, State: "tilled", GrowthModifier: 1}
	}
	f.Life.Plots[plot.ID] = plot
	return nil
}
