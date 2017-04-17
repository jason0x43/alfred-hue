package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	alfred "github.com/jason0x43/go-alfred"
	hue "github.com/jason0x43/go-hue"
)

// LevelCommand changes light levels
type LevelCommand struct{}

// About returns information about this command
func (c LevelCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "level",
		Description: "Change the light level for all lights",
		IsEnabled:   config.IPAddress != "" && config.Username != "",
	}
}

// Items returns the list of lights that can be adjusted
func (c LevelCommand) Items(arg, data string) (items []alfred.Item, err error) {
	var lights map[string]hue.Light

	session := hue.OpenSession(config.IPAddress, config.Username)
	if lights, err = session.Lights(); err != nil {
		return
	}

	var total int
	var count int

	for _, light := range lights {
		if light.State.On {
			count++
			total += light.State.Brightness
		}
	}

	if count == 0 {
		items = append(items, alfred.Item{
			Title: fmt.Sprintf("No lights are currently on"),
		})
		return
	}

	average := float64(total) / float64(count)
	dlog.Printf("Total brightness: %d", total)
	dlog.Printf("Light count: %d", count)
	dlog.Printf("Average brightness: %f", average)
	level := int(average)

	if arg == "" {
		items = append(items, alfred.Item{
			Title: fmt.Sprintf("Level: %v", level),
		})
	} else {
		item := alfred.Item{
			Title: "Level: " + arg,
		}

		var val int
		if val, err = strconv.Atoi(arg); err == nil && val >= 0 && val < 256 {
			item.Arg = &alfred.ItemArg{
				Keyword: "level",
				Mode:    alfred.ModeDo,
				Data:    alfred.Stringify(lightConfig{State: &hue.LightState{Brightness: val}}),
			}
		} else {
			item.Subtitle = "Enter an integer between 0 and 255"
		}

		items = append(items, item)
	}

	return items, nil
}

// Do sets the level for one or more lights
func (c LevelCommand) Do(data string) (out string, err error) {
	var cfg lightConfig

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshaling tag data: %v", err)
		}
	}

	session := hue.OpenSession(config.IPAddress, config.Username)
	lights, _ := session.Lights()

	id := cfg.Light

	for _, light := range lights {
		if id == "" || light.ID == id {
			if light.State.On {
				light.State.Brightness = (*cfg.State).Brightness
				if err := session.SetLightState(light.ID, light.State); err != nil {
					dlog.Printf("Error setting state for %v\n", light.ID)
				}
			}
		}
	}

	return
}
