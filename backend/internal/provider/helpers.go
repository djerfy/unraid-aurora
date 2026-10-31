package provider

import "math"

// historyLength is one hour of samples at the hub's two-second cadence.
const historyLength = 1800

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func diskColor(typ, smart string) string {
	if smart == "warning" {
		return "#FFB020"
	}
	switch typ {
	case "Parity":
		return "#9B7BFF"
	case "Cache":
		return "#5AA9FF"
	case "Flash":
		return "#3DDC97"
	default:
		return "#FF6B4A"
	}
}
