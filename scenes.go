package main

import (
	"strings"

	alfred "github.com/jason0x43/go-alfred"
	hue "github.com/jason0x43/go-hue"
)

// SceneCommand lists and selects scenes
type SceneCommand struct{}

// About returns information about this command
func (c SceneCommand) About() alfred.CommandDef {
	return alfred.CommandDef{
		Keyword:     "scenes",
		Description: "Choose a scene",
		IsEnabled:   config.IPAddress != "" && config.Username != "" && len(cache.Scenes) > 1,
	}
}

// Items returns the list of scenes that can be selected
func (c SceneCommand) Items(arg, data string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var sceneMap = map[string]hue.Scene{}

	for _, scene := range cache.Scenes {
		if scene.Owner != "none" && alfred.FuzzyMatches(scene.ShortName, arg) {
			fullName := scene.ShortName + ":" + strings.Join(scene.Lights, ",")
			sceneMap[fullName] = scene
		}
	}

	for _, scene := range sceneMap {
		lights, _ := getLightNamesFromIDs(scene.Lights)
		items = append(items, alfred.Item{
			Title:        scene.ShortName,
			Subtitle:     strings.Join(lights, ", "),
			Autocomplete: scene.ShortName,
			Arg: &alfred.ItemArg{
				Keyword: "scenes",
				Mode:    alfred.ModeDo,
				Data:    scene.ID,
			},
		})
	}

	return
}

// Do runs the command
func (c SceneCommand) Do(data string) (out string, err error) {
	session := hue.OpenSession(config.IPAddress, config.Username)
	err = session.SetScene(data)
	return
}
