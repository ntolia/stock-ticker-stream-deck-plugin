package main

import (
	"encoding/json"
	"image/color"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shayne/go-streamdeck-sdk"
	"github.com/shayne/stock-ticker-stream-deck-plugin/pkg/api"
)

type tile struct {
	context string
	title   string
	symbol  string
}

type plugin struct {
	sd    *streamdeck.StreamDeck
	tiles map[string]*tile
	state bool
	index map[string]string
}

type evSdpiCollection struct {
	Group     bool     `json:"group"`
	Index     int      `json:"index"`
	Key       string   `json:"key"`
	Selection []string `json:"selection"`
	Value     string   `json:"value"`
}

type settingsType map[string]string

func newPlugin(port, uuid, event, info string) *plugin {
	sd := streamdeck.NewStreamDeck(port, uuid, event, info)
	index := map[string]string{
		"^GSPC": "ES=F",  // S&P500
		"^DJI":  "YM=F",  // Dow
		"^IXIC": "NQ=F",  // NASDAQ
		"^RUT":  "RTY=F", // Russell 2000
	}
	p := &plugin{sd: sd, tiles: make(map[string]*tile), state: true, index: index}
	sd.SetDelegate(p)
	return p
}

func (p *plugin) renderTile(t *tile, data api.Result, futureData api.Result) *[]byte {
	var price, change, changePercent float64
	var status string
	statusColor := orange // regular/pre
	switch data.MarketState {
	case "REGULAR":
		status = ""
		price = data.RegularMarketPrice
		change = data.RegularMarketChange
		changePercent = data.RegularMarketChangePercent
	case "POST", "POSTPOST", "PREPRE", "CLOSED":
		statusColor = blue
		status = ""
		// Swap between regular and pre
		if p.state {
			status = ""
			statusColor = &color.RGBA{0x41, 0x69, 0xe1, 0xff} // Royal blue
			price = data.RegularMarketPrice
			change = data.RegularMarketChange
			changePercent = data.RegularMarketChangePercent
		} else {
			price = data.PostMarketPrice
			change = data.PostMarketChange
			changePercent = data.PostMarketChangePercent
			// TODO: Figure out what happens when the futures market isn't ready
			// If we have future data (likely index), use that instead
			if (api.Result{}) != futureData {
				price = futureData.RegularMarketPrice
				change = futureData.RegularMarketChange
				changePercent = futureData.RegularMarketChangePercent
			}
		}
	case "PRE":
		status = ""
		if data.PreMarketPrice > 0 {
			price = data.PreMarketPrice
			change = data.PreMarketChange
		} else {
			price = data.PostMarketPrice
			change = data.PostMarketChange
		}
		changePercent = data.PreMarketChangePercent
	}
	arrow := ""
	arrowColor := red
	if change > 0 {
		arrow = ""
		arrowColor = green
	} else if change == 0 {
		arrow = ""
	}
	// Use of index futures should hopefully be overriden by a user-provided tile title
	title := data.Symbol
	if t.title != "" {
		title = t.title
	}
	return DrawTile(title, price, change, changePercent, status, statusColor, arrow, arrowColor)
}

func (p *plugin) updateTiles(tiles []*tile) {
	var symbols []string
	for _, t := range tiles {
		if t.symbol != "" {
			symbols = append(symbols, t.symbol)
			// Append futures if we are tracking an index
			index_future := p.index[t.symbol]
			if index_future != "" {
				symbols = append(symbols, index_future)
			}
		}
	}
	if len(symbols) == 0 {
		return
	}
	stocks := api.Call(symbols)
	if stocks == nil {
		return
	}
	for _, t := range tiles {
		b := p.renderTile(t, stocks[t.symbol], stocks[p.index[t.symbol]])
		err := p.sd.SetImage(t.context, *b)
		if err != nil {
			log.Fatalf("sd.SetImage: %v\n", err)
		}
	}
}

func (p *plugin) startUpdateLoop() {
	tick := time.Tick(5 * time.Second)
	for {
		select {
		case <-tick:
			p.state = !p.state
			var tiles []*tile
			for _, t := range p.tiles {
				tiles = append(tiles, t)
			}
			p.updateTiles(tiles)
		}
	}
}

func (p *plugin) Run() {
	err := p.sd.Connect()
	if err != nil {
		log.Fatalf("Connect: %v\n", err)
	}
	go p.startUpdateLoop()
	p.sd.ListenAndWait()
}

func (p plugin) OnConnected(*websocket.Conn) {
}

func (p plugin) OnWillAppear(ev *streamdeck.EvWillAppear) {
	if t, ok := p.tiles[ev.Context]; ok {
		p.updateTiles([]*tile{t})
	} else {
		var settings settingsType
		err := json.Unmarshal(*ev.Payload.Settings, &settings)
		if err != nil {
			log.Println("OnWillAppear settings unmarshal", err)
		}
		t := &tile{context: ev.Context, symbol: settings["symbol"]}
		p.tiles[ev.Context] = t
		p.updateTiles([]*tile{t})
	}
}

func (p plugin) OnTitleParametersDidChange(ev *streamdeck.EvTitleParametersDidChange) {
	t := p.tiles[ev.Context]
	if t == nil {
		log.Println("OnTitleParametersDidChange: Tile not found")
		return
	}
	t.title = ev.Payload.Title
	p.updateTiles([]*tile{t})
}

func (p plugin) OnPropertyInspectorConnected(ev *streamdeck.EvSendToPlugin) {
	if t, ok := p.tiles[ev.Context]; ok {
		settings := make(settingsType)
		settings["symbol"] = t.symbol
		p.sd.SendToPropertyInspector(ev.Action, ev.Context, &settings)
	}
}

func (p plugin) OnSendToPlugin(ev *streamdeck.EvSendToPlugin) {
	var payload map[string]*json.RawMessage
	err := json.Unmarshal(*ev.Payload, &payload)
	if err != nil {
		log.Println("OnSendToPlugin unmarshal", err)
	}
	if data, ok := payload["sdpi_collection"]; ok {
		sdpi := evSdpiCollection{}
		err = json.Unmarshal(*data, &sdpi)
		if err != nil {
			log.Println("SDPI unmarshal", err)
		}
		switch sdpi.Key {
		case "symbol":
			symbol := strings.ToUpper(sdpi.Value)
			settings := make(settingsType)
			settings["symbol"] = symbol
			err = p.sd.SetSettings(ev.Context, &settings)
			if err != nil {
				log.Fatalf("setSettings: %v", err)
			}
			t := p.tiles[ev.Context]
			if t == nil {
				log.Fatal("Tile was nil")
			}
			t.symbol = symbol
			p.updateTiles([]*tile{t})
		}
	}
}

func (p plugin) OnApplicationDidLaunch(*streamdeck.EvApplication) {

}

func (p plugin) OnApplicationDidTerminate(*streamdeck.EvApplication) {

}
