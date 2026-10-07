package main

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:embed freetower_world.json
var freeTowerConfigJSON []byte

type freeTowerWorldConfig struct {
	Bound      float64                                                          `json:"bound"`
	Spawn      Vec2                                                             `json:"spawn"`
	WaterLevel float64                                                          `json:"waterLevel"`
	River      struct{ Center, Amplitude, Frequency, HalfWidth, Depth float64 } `json:"river"`
	Ocean      struct{ Shore, SlopeStart, SlopeEnd, Slope float64 }             `json:"ocean"`
	Movement   struct{ Walk, Run, Wade, Swim, SwimDepth, FloatOffset float64 }  `json:"movement"`
}

var freeTowerConfig = func() freeTowerWorldConfig {
	var c freeTowerWorldConfig
	if err := json.Unmarshal(freeTowerConfigJSON, &c); err != nil {
		panic(err)
	}
	return c
}()

func ftSmooth(a, b, n float64) float64       { t := clamp((n-a)/(b-a), 0, 1); return t * t * (3 - 2*t) }
func freeTowerCoastOffset(x float64) float64 { return math.Sin(x*.06)*3.5 + (math.Cos(x*.031)-1)*2 }
func freeTowerHeight(p Vec2) float64 {
	c := freeTowerConfig
	h := 2 + 12*math.Exp(-(math.Pow(p.X-34, 2)/260+math.Pow(p.Z+22, 2)/380)) + 6*math.Exp(-(p.X*p.X/500+math.Pow(p.Z+57, 2)/130)) + .22*math.Sin(p.X*.13)*math.Cos(p.Z*.11)
	d := math.Abs(p.Z - (c.River.Center + math.Sin(p.X*c.River.Frequency)*c.River.Amplitude))
	if d < c.River.HalfWidth {
		h = math.Min(h, -c.River.Depth+4.4*ftSmooth(0, c.River.HalfWidth, d))
	} else if d < c.River.HalfWidth+5 {
		h = 2 + (h-2)*ftSmooth(c.River.HalfWidth, c.River.HalfWidth+5, d)
	}
	coastalZ := p.Z - freeTowerCoastOffset(p.X)
	if coastalZ > c.Ocean.SlopeStart {
		h = math.Min(h, 2*(1-ftSmooth(c.Ocean.SlopeStart, c.Ocean.SlopeEnd, coastalZ))-math.Max(0, coastalZ-c.Ocean.SlopeEnd)*c.Ocean.Slope)
	}
	return h
}
func freeTowerLocomotion(p Vec2) (string, float64) {
	c := freeTowerConfig
	h := freeTowerHeight(p)
	inWater := p.Z >= c.Ocean.Shore+freeTowerCoastOffset(p.X) || math.Abs(p.Z-(c.River.Center+math.Sin(p.X*c.River.Frequency)*c.River.Amplitude)) < c.River.HalfWidth
	if inWater && h < c.WaterLevel {
		if c.WaterLevel-h > c.Movement.SwimDepth {
			return "swimming", c.WaterLevel - c.Movement.FloatOffset
		}
		return "shallow-water", h
	}
	return "land", h
}
func freeTowerPosition(p Vec2) Vec2 {
	return Vec2{clamp(p.X, -freeTowerConfig.Bound, freeTowerConfig.Bound), clamp(p.Z, -freeTowerConfig.Bound, freeTowerConfig.Bound)}
}
