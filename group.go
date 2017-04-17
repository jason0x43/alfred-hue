package main

import (
	"encoding/json"
	"strings"

	alfred "github.com/jason0x43/go-alfred"
)

// GroupCommand lists and updates groups on the hub
type GroupCommand struct{}

// About returns information about this command
func (c GroupCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "groups",
		Description: "See light groups",
		IsEnabled:   config.IPAddress != "" && config.Username != "" && len(cache.Groups) > 1,
	}
}

// Items returns the list of groups on the hub
func (c GroupCommand) Items(arg, data string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var cfg groupConfig

	if data != "" {
		if err = json.Unmarshal([]byte(data), &cfg); err != nil {
			dlog.Printf("Error unmarshalling group config: %v", err)
		}
	}

	if cfg.Group != "" {
	} else {
		for _, group := range cache.Groups {
			if alfred.FuzzyMatches(group.Name, arg) {
				lights, _ := getLightNamesFromIDs(group.Lights)

				items = append(items, alfred.Item{
					Title:    group.Name,
					Subtitle: strings.Join(lights, ", "),
				})
			}
		}
	}

	return
}

type groupConfig struct {
	Group string
}
