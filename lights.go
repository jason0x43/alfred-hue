package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-hue"
)

// LightCommand lists and updates light
type LightCommand struct{}

// About returns information about this command
func (c LightCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "lights",
		Description: "Control individual lights",
		IsEnabled:   config.IPAddress != "" && config.Username != "",
	}
}

// Items returns the list of lights known to the hub
func (c LightCommand) Items(arg, data string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var cfg lightConfig

	if data != "" {
		if err = json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling light config: %v", err)
		}
	}

	lid := cfg.Light
	lights := cache.Lights

	if lid == "" {
		for _, light := range lights {
			name := fmt.Sprintf("%s: %s", light.ID, light.Name)

			if alfred.FuzzyMatches(name, arg) {
				r, g, b := light.GetColorRGB()
				item := alfred.Item{
					Title:        name,
					Subtitle:     fmt.Sprintf("Hue: %d, Sat: %d, Bri: %d, RGB: #%02x%02x%02x", light.State.Hue, light.State.Saturation, light.State.Brightness, r, g, b),
					Icon:         "off.png",
					Autocomplete: light.ID,
					Arg: &alfred.ItemArg{
						Keyword: "lights",
						Data:    alfred.Stringify(lightConfig{Light: light.ID}),
					},
				}

				newState := "on"

				if light.State.On {
					item.Icon = "on.png"
					newState = "off"
				}

				item.AddMod(alfred.ModCmd, alfred.ItemMod{
					Subtitle: fmt.Sprintf("Turn light %s", newState),
					Arg: &alfred.ItemArg{
						Keyword: "lights",
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(lightConfig{Light: light.ID, State: &hue.LightState{On: !light.State.On}}),
					},
				})

				items = append(items, item)
			}
		}
	} else {
		light, _ := lights[lid]
		parts := alfred.CleanSplitN(arg, " ", 2)
		property := parts[0]

		if alfred.FuzzyMatches("name:", property) {
			var item alfred.Item

			if len(parts) > 1 {
				newName := parts[1]

				item.Title = "Name: " + newName
				item.Subtitle = "Name: " + light.Name
				item.Arg = &alfred.ItemArg{
					Keyword: "lights",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(lightConfig{Light: light.ID, Name: newName}),
				}
			} else {
				item.Title = "Name: " + light.Name
				item.Subtitle = "Update this light’s name"
				item.Autocomplete = "Name: " + light.Name
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("state:", property) {
			onOff := "on"
			state := "off"

			if light.State.On {
				onOff = "off"
				state = "on"
			}

			newState := light.State
			newState.On = !light.State.On

			items = append(items, alfred.Item{
				Title:    "State: " + state,
				Subtitle: "Press Enter to turn this light " + onOff,
				Arg: &alfred.ItemArg{
					Keyword: "lights",
					Mode:    alfred.ModeDo,
					Data:    alfred.Stringify(lightConfig{Light: light.ID, State: &newState}),
				},
			})
		}

		if alfred.FuzzyMatches("level:", property) {
			var item alfred.Item
			if len(parts) > 1 {
				if val, err := strconv.Atoi(parts[1]); err == nil {
					newState := light.State
					newState.Brightness = val

					item.Title = fmt.Sprintf("Level: %s", parts[1])
					item.Subtitle = fmt.Sprintf("Level: %d", light.State.Brightness)
					item.Arg = &alfred.ItemArg{
						Keyword: "lights",
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(lightConfig{Light: light.ID, State: &newState}),
					}
				} else {
					item.Title = fmt.Sprintf("Level: %s", parts[1])
					item.Subtitle = fmt.Sprintf("Invalid number '%s'", parts[1])
				}
			} else {
				item.Title = fmt.Sprintf("Level: %d", light.State.Brightness)
				item.Subtitle = "Set this light’s brightness"
				item.Autocomplete = fmt.Sprintf("Level: %d", light.State.Brightness)
			}

			items = append(items, item)
		}

		if alfred.FuzzyMatches("color:", property) {
			r, g, b := light.GetColorRGB()

			item := alfred.Item{
				Title:        fmt.Sprintf("Color: #%02x%02x%02x", r, g, b),
				Autocomplete: fmt.Sprintf("Color: #%02x%02x%02x", r, g, b),
			}

			if len(parts) > 1 {
				newLight := light

				if err = newLight.SetColorHex(parts[1]); err != nil {
					item.Subtitle = fmt.Sprintf("Invalid color '%s'", parts[1])
				} else {
					dlog.Printf("set new light color to %s", parts[1])
					nr, ng, nb := newLight.GetColorRGB()
					dlog.Printf("new b: %x", nb)
					item.Title = fmt.Sprintf("Color: #%02x%02x%02x", nr, ng, nb)
					item.Subtitle = fmt.Sprintf("Color: #%02x%02x%02x", r, g, b)
					item.Arg = &alfred.ItemArg{
						Keyword: "lights",
						Mode:    alfred.ModeDo,
						Data:    alfred.Stringify(lightConfig{Light: light.ID, State: &newLight.State}),
					}
				}
			} else {
				item.Subtitle = "Change this light’s color"
			}

			items = append(items, item)
		}
	}

	return
}

// Do runs the command
func (c LightCommand) Do(data string) (out string, err error) {
	var cfg lightConfig

	if data != "" {
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshaling tag data: %v", err)
		}
	}

	session := hue.OpenSession(config.IPAddress, config.Username)

	if cfg.State != nil {
		if err = session.SetLightState(cfg.Light, *cfg.State); err != nil {
			return
		}
		out = fmt.Sprintf("Set state for %s to %v", cfg.Light, cfg.State)
	}

	if cfg.Name != "" {
		if err = session.SetLightName(cfg.Light, cfg.Name); err != nil {
			return
		}
		if out == "" {
			out = fmt.Sprintf(", renamed %s to %s", cfg.Light, cfg.Name)
		} else {
			out = fmt.Sprintf("Set name of %s to %s", cfg.Light, cfg.Name)
		}
	}

	cache.LastUpdate = time.Time{}
	alfred.SaveJSON(cacheFile, &cache)
	dlog.Printf("cleared cache")

	return
}

type lightConfig struct {
	Light string
	State *hue.LightState
	Name  string
}
