package main

import (
	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-hue"
)

// HubCommand is for listing and binding to hubs
type HubCommand struct{}

// About returns information about this command
func (c HubCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "hub",
		Description: "Select a Hue hub to connect to",
		IsEnabled:   true,
	}
}

// Items returns the list of found hubs
func (c HubCommand) Items(arg, data string) (items []alfred.Item, err error) {
	var hubs []hue.Hub

	if hubs, err = hue.GetHubs(); err != nil {
		return
	}

	for _, h := range hubs {
		title := h.Name
		if title == "" {
			title = h.IPAddress
		}
		items = append(items, alfred.Item{
			Title:        title,
			Autocomplete: h.Name,
			Arg: &alfred.ItemArg{
				Keyword: "hub",
				Mode:    alfred.ModeDo,
				Data:    h.IPAddress,
			},
		})
	}

	return
}

// Do runs the command
func (c HubCommand) Do(data string) (out string, err error) {
	if err = workflow.ShowMessage("Press the button on your hub, then click OK to continue..."); err != nil {
		return
	}

	var session hue.Session
	if session, err = hue.NewSession(data); err != nil {
		workflow.ShowMessage("There was an error accessing your hub:\n\n" + err.Error())
		return
	}

	config.IPAddress = session.IPAddress()
	config.Username = session.Username()
	workflow.ShowMessage("You've successfully connected to your hub!")

	err = alfred.SaveJSON(configFile, &config)
	return
}
