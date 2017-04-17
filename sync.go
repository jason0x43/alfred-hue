package main

import alfred "github.com/jason0x43/go-alfred"

// SyncCommand syncs light and scene data with the hub
type SyncCommand struct{}

// About returns information about this command
func (c SyncCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "sync",
		Description: "Refresh light and scene data from the hub",
		IsEnabled:   config.IPAddress != "" && config.Username != "",
	}
}

// Items returns the status of the sync command
func (c SyncCommand) Items(arg, data string) (items []alfred.Item, err error) {
	// TODO: Another option would be to "learn" the scenes by setting them
	// using group/0 and retrieving the light details. Afterwords use the light
	// details to set scenes.

	if err = refresh(); err != nil {
		return
	}

	items = append(items, alfred.Item{
		Title: "Refreshed!",
	})

	return
}
