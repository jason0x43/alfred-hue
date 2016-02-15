package main

import (
	"encoding/json"
	"fmt"
	_log "log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-hue"
)

var log = _log.New(os.Stderr, "[hue] ", _log.LstdFlags)

type hueConfig struct {
	IPAddress string
	Username  string
	APIToken  string
}

type hueCache struct {
	LastUpdate    time.Time
	Lights        map[string]hue.Light
	Scenes        map[string]hue.Scene
	MeetHueScenes map[string]hue.MeetHueScene
	Groups        map[string]hue.Group
	LastMHUpdate  time.Time
	MHScenes      []hue.MeetHueScene
}

var configFile string
var cacheFile string
var config hueConfig
var cache hueCache
var workflow alfred.Workflow

func main() {
	var err error
	if workflow, err = alfred.OpenWorkflow(".", true); err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	log.Println("Using config file", configFile)
	log.Println("Using cache file", cacheFile)

	if err = alfred.LoadJSON(configFile, &config); err != nil {
		log.Println("Error loading config:", err)
	}

	if err = alfred.LoadJSON(cacheFile, &cache); err != nil {
		log.Println("Error loading cache:", err)
	}

	commands := []alfred.Command{
		SceneCommand(0),
		GroupCommand(0),
		LightCommand(0),
		LevelCommand(0),
		SyncCommand(0),
		HubCommand(0),
		// MeetHueCommand(0),
	}

	workflow.Run(commands)
}

// hub ---------------------------------

// HubCommand is for listing and binding to hubs
type HubCommand int

// Keyword returns the command keyword
func (c HubCommand) Keyword() string {
	return "hub"
}

// IsEnabled indicates whether the command is enabled
func (c HubCommand) IsEnabled() bool {
	return true
}

// MenuItem returns the Alfred menu item for the command
func (c HubCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "", " ", "Select a Hue hub to connect to")
}

// Items returns the list of found hubs
func (c HubCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item

	hubs, err := hue.GetHubs()
	if err != nil {
		return items, err
	}

	for _, h := range hubs {
		title := h.Name
		if title == "" {
			title = h.IPAddress
		}
		items = append(items, alfred.Item{
			Title:        title,
			Arg:          "hub " + h.IPAddress,
			Autocomplete: prefix + h.Name,
		})
	}

	return items, nil
}

// Do binds to a specific hub
func (c HubCommand) Do(query string) (string, error) {
	err := workflow.ShowMessage("Press the button on your hub, then click OK to continue...")
	if err != nil {
		return "", err
	}

	ipAddress := query
	session, err := hue.NewSession(ipAddress, "jason0x43-alfred-hue")

	if err != nil {
		workflow.ShowMessage("There was an error accessing your hub:\n\n" + err.Error())
		return "", err
	}

	config.IPAddress = session.IPAddress()
	config.Username = session.Username()
	workflow.ShowMessage("You've successfully connected to your hub!")

	err = alfred.SaveJSON(configFile, &config)
	return "", err
}

// level -------------------------------

// LevelCommand changes light levels
type LevelCommand int

// Keyword returns the level command keyword
func (c LevelCommand) Keyword() string {
	return "level"
}

// IsEnabled indicates whether the command is enabled
func (c LevelCommand) IsEnabled() bool {
	return config.IPAddress != "" && config.Username != ""
}

// MenuItem returns the Alfred menu item for the command
func (c LevelCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		Subtitle:     "Change the light level for all lights",
	}
}

// Items returns the list of lights that can be adjusted
func (c LevelCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item

	session := hue.OpenSession(config.IPAddress, config.Username)
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
	log.Printf("Total brightness: %d", total)
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

// Do sets the level for one or more lights
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

	session := hue.OpenSession(config.IPAddress, config.Username)
	lights, _ := session.Lights()

	for _, light := range lights {
		if id == "" || light.ID == id {
			if light.State.On {
				light.State.Brightness = level
				err := session.SetLightState(light.ID, light.State)
				if err != nil {
					log.Printf("Error setting state for %v\n", light.ID)
				}
			}
		}
	}

	return "", nil
}

// scene -------------------------------

// SceneCommand lists and selects scenes
type SceneCommand int

// Keyword returns the command keyword
func (c SceneCommand) Keyword() string {
	return "scenes"
}

// IsEnabled indicates whether the command is enabled
func (c SceneCommand) IsEnabled() bool {
	return config.IPAddress != "" && config.Username != "" && len(cache.Scenes) > 1
}

// MenuItem returns the Alfred menu item for the command
func (c SceneCommand) MenuItem() alfred.Item {
	return alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
		Subtitle:     "Choose a scene",
	}
}

// Items returns the list of scenes that can be selected
func (c SceneCommand) Items(prefix, query string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	var sceneMap = map[string]hue.Scene{}

	for _, scene := range cache.Scenes {
		if scene.Owner != "none" && alfred.FuzzyMatches(scene.ShortName, query) {
			fullName := scene.ShortName + ":" + strings.Join(scene.Lights, ",")
			sceneMap[fullName] = scene
		}
	}

	for _, scene := range sceneMap {
		lights, _ := getLightNamesFromIDs(scene.Lights)
		items = append(items, alfred.Item{
			Title:        scene.ShortName,
			SubtitleAll:  strings.Join(lights, ", "),
			Autocomplete: prefix + scene.ShortName,
			Arg:          "scenes " + scene.ID,
		})
	}

	items = alfred.SortItemsForKeyword(items, query)
	return
}

// Do selects a scene
func (c SceneCommand) Do(query string) (out string, err error) {
	session := hue.OpenSession(config.IPAddress, config.Username)
	err = session.SetScene(query)
	return
}

// group -------------------------------

// GroupCommand lists and updates groups on the hub
type GroupCommand int

// Keyword returns the command keyword
func (c GroupCommand) Keyword() string {
	return "groups"
}

// IsEnabled indicates whether the command is enabled
func (c GroupCommand) IsEnabled() bool {
	return config.IPAddress != "" && config.Username != "" && len(cache.Groups) > 0
}

// MenuItem returns the Alfred menu item for the command
func (c GroupCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "", "", "See light groups")
}

// Items returns the list of groups on the hub
func (c GroupCommand) Items(prefix, query string) (items []alfred.Item, err error) {
	if err = checkRefresh(); err != nil {
		return
	}

	for _, group := range cache.Groups {
		if alfred.FuzzyMatches(group.Name, query) {
			lights, _ := getLightNamesFromIDs(group.Lights)

			items = append(items, alfred.Item{
				Title:       group.Name,
				SubtitleAll: strings.Join(lights, ", "),
				Valid:       alfred.Invalid,
			})
		}
	}

	sort.Sort(alfred.ByTitle(items))
	return
}

// Light -------------------------------

// LightCommand lists and updates light
type LightCommand int

// Keyword returns the command keyword
func (c LightCommand) Keyword() string {
	return "lights"
}

// IsEnabled indicates whether the command is enabled
func (c LightCommand) IsEnabled() bool {
	return config.IPAddress != "" && config.Username != ""
}

// MenuItem returns the Alfred menu item for the command
func (c LightCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "", " ", "Control individual lights")
}

// Items returns the list of lights known to the hub
func (c LightCommand) Items(prefix, query string) ([]alfred.Item, error) {
	var items []alfred.Item
	session := hue.OpenSession(config.IPAddress, config.Username)
	lights, _ := session.Lights()
	parts := alfred.TrimAllLeft(strings.Split(query, alfred.Separator))

	if len(parts) == 1 {
		for _, light := range lights {
			name := fmt.Sprintf("%s: %s", light.ID, light.Name)

			if alfred.FuzzyMatches(name, query) {
				state, _ := json.Marshal(&hue.LightState{On: !light.State.On})
				item := alfred.Item{
					Title:        name,
					SubtitleAll:  fmt.Sprintf("Hue: %d, Sat: %d, Bri: %d", light.State.Hue, light.State.Saturation, light.State.Brightness),
					Arg:          fmt.Sprintf("%s %s %s", c.Keyword(), light.ID, state),
					Icon:         "off.png",
					Autocomplete: prefix + light.ID + alfred.Separator + " ",
				}

				if light.State.On {
					item.Icon = "on.png"
				}

				items = append(items, item)
			}
		}

		alfred.SortItemsForKeyword(items, query)
	} else {
		id := parts[0]
		light, _ := cache.Lights[id]
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

			// if alfred.FuzzyMatches("color", query) {
			// 	r, g, b := light.GetColorRGB()
			// 	items = append(items, alfred.Item{
			// 		Title:        fmt.Sprintf("Color: rgb(%d,%d,%d)", light.State.Brightness),
			// 		Icon:         icon,
			// 		Valid:        alfred.Invalid,
			// 		Autocomplete: prefix + "Color" + alfred.Separator + " ",
			// 	})
			// }
		}
	}

	return items, nil
}

// Do updates a light
func (c LightCommand) Do(query string) (string, error) {
	session := hue.OpenSession(config.IPAddress, config.Username)
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

// SyncCommand syncs light and scene data with the hub
type SyncCommand int

// Keyword returns the command keyword
func (c SyncCommand) Keyword() string {
	return "sync"
}

// IsEnabled incidates whether the command is enabled
func (c SyncCommand) IsEnabled() bool {
	return config.Username != ""
}

// MenuItem returns the Alfred menu item for the command
func (c SyncCommand) MenuItem() alfred.Item {
	return alfred.NewKeywordItem(c.Keyword(), "", "", "Refresh light and scene data from the hub")
}

// Items returns the status of the sync command
func (c SyncCommand) Items(prefix, query string) (items []alfred.Item, err error) {
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

// MeetHue -----------------------------

// MeetHueCommand downloads scene data from meethue.com
type MeetHueCommand int

// Keyword returns the command keyword
func (c MeetHueCommand) Keyword() string {
	return "meethue"
}

// IsEnabled indicates whether the command is enabled
func (c MeetHueCommand) IsEnabled() bool {
	return config.Username != ""
}

// MenuItem returns the Alfred menu item for the command
func (c MeetHueCommand) MenuItem() alfred.Item {
	item := alfred.Item{
		Title:        c.Keyword(),
		Autocomplete: c.Keyword() + " ",
		Valid:        alfred.Invalid,
	}
	return item
}

// Items returns the status of the meethue command
func (c MeetHueCommand) Items(prefix, query string) ([]alfred.Item, error) {
	if config.APIToken == "" {
		item := alfred.Item{
			Title:        "login",
			Autocomplete: prefix + "login",
			SubtitleAll:  "Login to MeetHue.com to enable scene downloading",
			Arg:          "meethue login",
		}
		return []alfred.Item{item}, nil
	}

	item1 := alfred.Item{
		Title:        "download",
		Autocomplete: prefix + "download",
		SubtitleAll:  "Download scenes",
		Arg:          "meethue download",
	}
	item2 := alfred.Item{
		Title:        "logout",
		Autocomplete: prefix + "logout",
		SubtitleAll:  "Logout of MeetHue.com",
		Arg:          "meethue logout",
	}
	return []alfred.Item{item1, item2}, nil
}

// Do downloads data from meethue
func (c MeetHueCommand) Do(query string) (string, error) {
	switch query {
	case "login":
		btn, username, err := workflow.GetInput("Username", "", false)
		if err != nil {
			return "", err
		}

		if btn != "Ok" {
			log.Println("User didn't click OK")
			return "", nil
		}
		log.Printf("username: %s", username)

		btn, password, err := workflow.GetInput("Password", "", true)
		if btn != "Ok" {
			log.Println("User didn't click OK")
			return "", nil
		}
		log.Printf("password: *****")

		token, err := hue.GetMeetHueToken(username, password)
		if err != nil {
			workflow.ShowMessage("There was an error logging in:\n\n" + err.Error())
			return "", err
		}

		config.APIToken = token
		err = alfred.SaveJSON(configFile, &config)
		if err != nil {
			return "", err
		}

		workflow.ShowMessage("Login successful!")
	case "logout":
		config.APIToken = ""
		err := alfred.SaveJSON(configFile, &config)
		if err != nil {
			return "", err
		}

		workflow.ShowMessage("Logged out")
	case "download":
		session := hue.OpenMeetHueSession(config.APIToken)
		scenes, err := session.GetMeetHueScenes()
		if err != nil {
			return "", err
		}

		cache.MeetHueScenes = scenes
		err = alfred.SaveJSON(cacheFile, cache)
		if err != nil {
			log.Println("Error saving cache:", err)
		}

		return "Synchronized!", nil
	}

	return "", fmt.Errorf("Invalid command")
}

// Support functions -------------------

type byValue []string

func (b byValue) Len() int           { return len(b) }
func (b byValue) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byValue) Less(i, j int) bool { return b[i] < b[j] }

func getLightNamesFromIDs(ids []string) (names []string, err error) {
	for _, id := range ids {
		light, _ := cache.Lights[id]
		names = append(names, light.Name)
	}
	return
}

func refresh() (err error) {
	session := hue.OpenSession(config.IPAddress, config.Username)
	if cache.Lights, err = session.Lights(); err != nil {
		return
	}
	if cache.Scenes, err = session.Scenes(); err != nil {
		return
	}
	if cache.Groups, err = session.Groups(); err != nil {
		return
	}
	cache.LastUpdate = time.Now()
	return alfred.SaveJSON(cacheFile, &cache)
}

func checkRefresh() error {
	if time.Now().Sub(cache.LastUpdate) > time.Duration(1)*time.Minute {
		return refresh()
	}
	return nil
}
