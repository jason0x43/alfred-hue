package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-hue"
)

type Config struct {
	IpAddress string
	Username  string
	ApiToken  string
}

type Cache struct {
	Scenes []hue.MeetHueScene
}

var configFile string
var cacheFile string
var config Config
var cache Cache

func main() {
	workflow, err := alfred.OpenWorkflow(".", true)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	log.Println("Using config file", configFile)
	log.Println("Using cache file", cacheFile)

	err = alfred.LoadJson(configFile, &config)
	if err != nil {
		log.Println("Error loading config:", err)
	}

	alfred.LoadJson(cacheFile, &cache)
	if err != nil {
		log.Println("Error loading cache:", err)
	}

	commands := []alfred.Command{
		SceneCommand{workflow},
		LightCommand{workflow},
		LevelCommand{workflow},
		SyncCommand{workflow},
		HubCommand{workflow},
		MeetHueCommand{workflow},
	}

	workflow.Run(commands)
}

type Command struct {
	workflow *alfred.Workflow
}

// hub ---------------------------------

type HubCommand Command

func (c HubCommand) Keyword() string {
	return "hub"
}

func (c HubCommand) IsEnabled() bool {
	return true
}

func (t HubCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Valid:        alfred.Invalid,
		Subtitle:     "Select a Hue hub to connect to"}
}

func (c HubCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item

	hubs, err := hue.GetHubs()
	if err != nil {
		return items, err
	}

	for _, h := range hubs {
		items = append(items, alfred.Item{
			Title:        h.Name,
			Arg:          "hub " + h.IpAddress,
			Autocomplete: prefix + h.Name})
	}
	return items, nil
}

func (c HubCommand) Do(query string) (string, error) {
	err := c.workflow.ShowMessage("Press the button on your hub, then click OK to continue...")
	if err != nil {
		return "", err
	}

	ipAddress := query
	session, err := hue.NewSession(ipAddress, "jason0x43-alfred-hue")

	if err != nil {
		c.workflow.ShowMessage("There was an error accessing your hub:\n\n" + err.Error())
		return "", err
	} else {
		config.IpAddress = session.IpAddress()
		config.Username = session.Username()
		c.workflow.ShowMessage("You've successfully connected to your hub!")

		err = alfred.SaveJson(configFile, &config)
		return "", err
	}
}

// level -------------------------------

type LevelCommand Command

func (c LevelCommand) Keyword() string {
	return "level"
}

func (c LevelCommand) IsEnabled() bool {
	return config.IpAddress != "" && config.Username != ""
}

func (t LevelCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Valid:        alfred.Invalid,
		Subtitle:     "Change the light level for all lights"}
}

func (c LevelCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item

	session := hue.OpenSession(config.IpAddress, config.Username)
	lights, err := session.Lights()
	if err != nil {
		return items, err
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
			Valid: alfred.Invalid,
		})
		return items, nil
	}

	average := float64(total) / float64(count)
	log.Printf("Total brightness: %f", total)
	log.Printf("Light count: %d", count)
	log.Printf("Average brightness: %f", average)
	level := int(average)

	if query == "" {
		items = append(items, alfred.Item{
			Title: fmt.Sprintf("Level: %v", level),
			Valid: alfred.Invalid,
		})
	} else {
		item := alfred.Item{
			Title: "Level: " + query,
		}

		val, err := strconv.Atoi(query)
		if err == nil && val >= 0 && val < 256 {
			item.Arg = "level " + query
		} else {
			item.Valid = alfred.Invalid
			item.Subtitle = "Enter an integer between 0 and 255"
		}

		items = append(items, item)
	}

	return items, nil
}

func (c LevelCommand) Do(query string) (out string, err error) {
	parts := strings.SplitN(query, " ", 2)
	var level int
	var id string

	log.Printf("parts: %v", parts)

	if len(parts) == 2 {
		id = parts[0]

		level, err = strconv.Atoi(parts[1])
		if err != nil {
			return out, err
		}
	} else {
		level, err = strconv.Atoi(parts[0])
		if err != nil {
			return out, err
		}
	}

	session := hue.OpenSession(config.IpAddress, config.Username)
	lights, _ := session.Lights()

	for i, light := range lights {
		if id == "" || i == id {
			if light.State.On {
				light.State.Brightness = level
				err := session.SetLightState(light.Id, light.State)
				if err != nil {
					log.Printf("Error setting state for %v\n", light.Id)
				}
			}
		}
	}

	return "", nil
}

// scene -------------------------------

type SceneCommand Command

func (c SceneCommand) Keyword() string {
	return "scenes"
}

func (c SceneCommand) IsEnabled() bool {
	return config.IpAddress != "" && config.Username != "" && len(cache.Scenes) > 1
}

func (t SceneCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Valid:        alfred.Invalid,
		Subtitle:     "Choose a scene"}
}

func (c SceneCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	scenes := cache.Scenes

	for _, scene := range scenes {
		name := scene.Name
		matcher := fmt.Sprintf("%s %s %s", scene.Category, name, scene.Category)

		if alfred.FuzzyMatches(matcher, query) {
			lights := []string{}
			for _, light := range scene.Lights {
				lights = append(lights, light.Id)
			}
			sort.Sort(byValue(lights))

			item := alfred.Item{
				Title:        name,
				SubtitleAll:  fmt.Sprintf("%v", lights),
				Arg:          c.Keyword() + " " + scene.Id,
				Autocomplete: prefix + name}

			if scene.Category != "" {
				item.SubtitleAll = scene.Category + ", " + item.SubtitleAll
			}

			items = append(items, item)
		}
	}

	sort.Sort(alfred.ByTitle(items))

	return items, nil
}

func (c SceneCommand) Do(query string) (string, error) {
	for _, scene := range cache.Scenes {
		if scene.Id == query {
			log.Printf("Setting scene to " + scene.Id)
			session := hue.OpenSession(config.IpAddress, config.Username)
			for _, state := range scene.Lights {
				ls := state.ToLightState()
				if scene.Recipe != "" {
					ls.On = true
				}
				err := session.SetLightState(state.Id, ls)
				if err != nil {
					return "", err
				}
			}
			return "", nil
		}
	}

	return "", fmt.Errorf("Invaid scene %s", query)
}

// Light -------------------------------

type LightCommand Command

func (c LightCommand) Keyword() string {
	return "lights"
}

func (c LightCommand) IsEnabled() bool {
	return config.IpAddress != "" && config.Username != ""
}

func (t LightCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword() + " ",
		Valid:        alfred.Invalid,
		Subtitle:     "Control individual lights"}
}

func (c LightCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	session := hue.OpenSession(config.IpAddress, config.Username)
	lights, _ := session.Lights()
	parts := alfred.SplitAndTrimQuery(query)

	if len(parts) == 1 {
		for id, light := range lights {
			name := fmt.Sprintf("%s: %s", id, light.Name)

			if alfred.FuzzyMatches(name, query) {
				state, _ := json.Marshal(&hue.LightState{On: !light.State.On})
				item := alfred.Item{
					Title:        name,
					Arg:          fmt.Sprintf("%s %s %s", c.Keyword(), id, state),
					Icon:         "off.png",
					Autocomplete: prefix + id + alfred.Separator + " "}

				if light.State.On {
					item.Icon = "on.png"
				}

				items = append(items, item)
			}
		}

		sort.Sort(alfred.ByTitle(items))
	} else {
		id := parts[0]
		light := lights[id]
		prefix += id + alfred.Separator + " "
		property := strings.ToLower(parts[1])

		log.Printf("id: %v", id)
		log.Printf("prop: %v", property)

		switch property {
		case "name":
			if len(parts) > 2 {
				query = parts[2]
			} else {
				query = ""
			}

			items = append(items, alfred.Item{
				Title: fmt.Sprintf("Set name to: %s", query),
			})
		case "on":
			var onOff string

			if light.State.On {
				onOff = "off"
			} else {
				onOff = "on"
			}

			state, _ := json.Marshal(&hue.LightState{On: !light.State.On})
			items = append(items, alfred.Item{
				Title: "Turn " + onOff + " " + light.Name,
				Arg:   fmt.Sprintf("%s %s %s", c.Keyword(), id, state),
			})
		case "level":
			items = append(items, alfred.Item{
				Title: "Set level to",
			})
			item := &items[len(items)-1]

			if len(parts) > 2 {
				item.Title += " " + parts[2]
			} else {
				item.Title += "..."
			}

			if _, err := strconv.Atoi(parts[2]); err == nil {
				item.Arg = fmt.Sprintf("level %s %s", id, parts[2])
			} else {
				item.Valid = alfred.Invalid
				item.SubtitleAll = fmt.Sprintf("Invalid number '%s'", parts[2])
			}
		default:
			query = parts[1]
			icon := "off.png"

			if light.State.On {
				icon = "on.png"
			}

			if alfred.FuzzyMatches("state", query) {
				state := "off"
				if light.State.On {
					state = "on"
				}
				newState, _ := json.Marshal(&hue.LightState{On: !light.State.On})
				items = append(items, alfred.Item{
					Title: "State: " + state,
					Icon:  icon,
					Arg:   fmt.Sprintf("%s %s %s", c.Keyword(), id, newState),
				})
			}

			if alfred.FuzzyMatches("name", query) {
				items = append(items, alfred.Item{
					Title:        fmt.Sprintf("Name: %s", light.Name),
					Icon:         icon,
					Valid:        alfred.Invalid,
					Autocomplete: prefix + "Name" + alfred.Separator + " ",
				})
			}

			if alfred.FuzzyMatches("level", query) {
				items = append(items, alfred.Item{
					Title:        fmt.Sprintf("Level: %d", light.State.Brightness),
					Icon:         icon,
					Valid:        alfred.Invalid,
					Autocomplete: prefix + "Level" + alfred.Separator + " ",
				})
			}
		}
	}

	return items, nil
}

func (c LightCommand) Do(query string) (string, error) {
	session := hue.OpenSession(config.IpAddress, config.Username)
	parts := strings.SplitN(query, " ", 2)
	log.Printf("Split '%s' into %s", query, parts)

	if len(parts) < 2 {
		return "", fmt.Errorf("Missing light state data")
	}

	var ls hue.LightState
	log.Printf("Unmarshalling JSON from %s\n", parts[1])
	err := json.Unmarshal([]byte(parts[1]), &ls)
	if err != nil {
		log.Printf("Error setting state: %s\n", err)
		return "", err
	}
	err = session.SetLightState(parts[0], ls)
	return fmt.Sprintf("Set state for %s to %s", parts[0], parts[1]), nil
}

// Sync --------------------------------

type SyncCommand Command

func (c SyncCommand) Keyword() string {
	return "sync"
}

func (c SyncCommand) IsEnabled() bool {
	return config.ApiToken != ""
}

func (t SyncCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword(),
		Valid:        alfred.Invalid,
		Subtitle:     "Download your scenes from MeetHue.com"}
}

func (c SyncCommand) Items(prefix, query string) ([]alfred.Item, error) {
	// TODO: Another option would be to "learn" the scenes by setting them
	// using group/0 and retrieving the light details. Afterwords use the light
	// details to set scenes.

	var items []alfred.Item
	session := hue.OpenMeetHueSession(config.ApiToken)
	scenes, err := session.GetMeetHueScenes()
	if err != nil {
		return items, err
	}

	cache.Scenes = scenes
	err = alfred.SaveJson(cacheFile, cache)
	if err != nil {
		log.Println("Error saving cache:", err)
	}

	items = append(items, alfred.Item{
		Title: "Synchronized!"})

	return items, nil
}

// MeetHue -----------------------------

type MeetHueCommand Command

func (c MeetHueCommand) Keyword() string {
	return "meethue"
}

func (c MeetHueCommand) IsEnabled() bool {
	return config.Username != ""
}

func (t MeetHueCommand) MenuItem() alfred.Item {
	item := alfred.Item{
		Title:        t.Keyword(),
		Autocomplete: t.Keyword(),
		Arg:          "meethue",
	}
	if config.ApiToken == "" {
		item.SubtitleAll = "Login to MeetHue.com to enable scene downloading"
	} else {
		item.SubtitleAll = "Logout of MeetHue.com"
	}
	return item
}

func (c MeetHueCommand) Items(prefix, query string) ([]alfred.Item, error) {
	return []alfred.Item{c.MenuItem()}, nil
}

func (c MeetHueCommand) Do(query string) (string, error) {
	if config.ApiToken == "" {
		btn, username, err := c.workflow.GetInput("Username", "", false)
		if err != nil {
			return "", err
		}

		if btn != "Ok" {
			log.Println("User didn't click OK")
			return "", nil
		}
		log.Printf("username: %s", username)

		btn, password, err := c.workflow.GetInput("Password", "", true)
		if btn != "Ok" {
			log.Println("User didn't click OK")
			return "", nil
		}
		log.Printf("password: *****")

		token, err := hue.GetMeetHueToken(username, password)
		if err != nil {
			c.workflow.ShowMessage("There was an error logging in:\n\n" + err.Error())
			return "", err
		}

		config.ApiToken = token
		err = alfred.SaveJson(configFile, &config)
		if err != nil {
			return "", err
		}

		c.workflow.ShowMessage("Login successful!")
	} else {
		config.ApiToken = ""
		err := alfred.SaveJson(configFile, &config)
		if err != nil {
			return "", err
		}

		c.workflow.ShowMessage("Logged out")
	}

	return "", nil
}

// Support functions -------------------

type byValue []string

func (b byValue) Len() int           { return len(b) }
func (b byValue) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byValue) Less(i, j int) bool { return b[i] < b[j] }
