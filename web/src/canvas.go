//go:build js && wasm

package main

import (
	"math"

	"github.com/z46-dev/dnscomms/serverless"
	"github.com/z46-dev/wasmdraw"
	"github.com/z46-dev/wasmdraw/ctx2d"
)

type point struct{ x, y float64 }

var (
	context *ctx2d.Context
	reduced bool
)

// attachCanvas installs one resize-aware renderer, independent of React form state.
func attachCanvas(reducedMotion bool) (result string) {
	reduced = reducedMotion
	if context != nil {
		return
	}
	var (
		canvas *wasmdraw.CanvasElement
		err    error
	)
	if canvas, err = wasmdraw.GetCanvasById("network-map", nil); err != nil {
		return err.Error()
	}
	if context, err = ctx2d.GetContext(canvas, nil); err != nil {
		return err.Error()
	}
	context.OnFrame = drawNetwork
	return
}

// drawNetwork advances the game loop and renders the live packet objects.
func drawNetwork(dt, width, height float64) {
	_ = network.Apply(serverless.Input{Action: "tick", Delta: min(dt, 0.1)})
	var (
		nodes map[string]point = map[string]point{
			"exfil-client": {width * 0.18, 70}, "client-1": {width * 0.5, 70}, "client-2": {width * 0.82, 70},
			"firewall": {width * 0.5, height * 0.3}, "orchestrator": {width * 0.5, height - 50},
		}
		colors map[string]string = map[string]string{"ordinary DNS": "#477cad", "exfil part": "#c37920", "cover response": "#537d63", "forwarded part": "#995cad"}
	)
	context.ClearRect(0, 0, width, height)
	context.FillStyle = "#edeae3"
	context.FillRect(8, height*0.3-3, width-16, 6)
	context.Font = "11px sans-serif"
	context.TextAlign = "left"
	context.FillStyle = "#817c72"
	context.FillText("INSIDE", 8, 24)
	context.FillText("OUTSIDE", 8, height*0.3+28)
	context.TextAlign = "center"
	for _, client := range []string{"exfil-client", "client-1", "client-2"} {
		line(nodes[client], nodes["firewall"], "#dedbd3")
	}
	for i, server := range network.Servers {
		var (
			columns  int = min(3, len(network.Servers))
			row      int = i / columns
			column   int = i % columns
			rowCount int = min(columns, len(network.Servers)-row*columns)
		)
		nodes[server.ID] = point{width * float64(column+1) / float64(rowCount+1), height * (0.52 + float64(row)*0.18)}
	}
	for _, server := range network.Servers {
		line(nodes["firewall"], nodes[server.ID], "#dedbd3")
		if server.Poisoned {
			line(nodes[server.ID], nodes["orchestrator"], "#cfbed8")
		}
		if server.ForwardTo != "" {
			line(nodes[server.ID], nodes[server.ForwardTo], "#8da6b9")
		}
	}
	for id, position := range nodes {
		var label string = id
		context.FillStyle = "#faf9f5"
		context.StrokeStyle = "#888276"
		if id == "exfil-client" {
			label = "Exfil client"
		}
		for _, server := range network.Servers {
			if server.ID == id {
				if server.Poisoned {
					context.StrokeStyle = "#995cad"
					label += " · exfil"
				}
			}
		}
		context.BeginPath().Arc(position.x, position.y, 10, 0, math.Pi*2).Fill()
		context.Stroke()
		context.FillStyle = "#292820"
		context.FillText(label, position.x, position.y+27, width*0.3)
	}
	for _, packet := range network.Packets {
		var (
			start    point
			end      point
			ok       bool
			progress float64 = packet.Progress
		)
		if start, ok = nodes[packet.Source]; !ok {
			continue
		}
		if end, ok = nodes[packet.Destination]; !ok {
			continue
		}
		if reduced {
			progress = 0
		}
		context.FillStyle = colors[packet.Classification]
		context.BeginPath().Arc(start.x+(end.x-start.x)*progress, start.y+(end.y-start.y)*progress, 5, 0, math.Pi*2).Fill()
	}

}

func line(start, end point, color string) {
	context.StrokeStyle = color
	context.BeginPath().MoveTo(start.x, start.y).LineTo(end.x, end.y).Stroke()
}
