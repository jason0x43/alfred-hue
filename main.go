package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/jason0x43/go-alfred"
	"github.com/jason0x43/go-hue"
)

var dlog = log.New(os.Stderr, "[hue] ", log.LstdFlags)

type hueConfig struct {
	IPAddress string
	Username  string
	APIToken  string
}

type hueCache struct {
	LastUpdate   time.Time
	Lights       map[string]hue.Light
	Scenes       map[string]hue.Scene
	Groups       map[string]hue.Group
	LastMHUpdate time.Time
}

var configFile string
var cacheFile string
var config hueConfig
var cache hueCache
var workflow alfred.Workflow

func main() {
	if !alfred.IsDebugging() {
		dlog.SetOutput(ioutil.Discard)
		dlog.SetFlags(0)
	}

	var err error
	if workflow, err = alfred.OpenWorkflow(".", true); err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	configFile = path.Join(workflow.DataDir(), "config.json")
	cacheFile = path.Join(workflow.CacheDir(), "cache.json")

	dlog.Println("Using config file", configFile)
	dlog.Println("Using cache file", cacheFile)

	if err = alfred.LoadJSON(configFile, &config); err != nil {
		dlog.Println("Error loading config:", err)
	}

	if err = alfred.LoadJSON(cacheFile, &cache); err != nil {
		dlog.Println("Error loading cache:", err)
	}

	workflow.Run([]alfred.Command{
		SceneCommand{},
		GroupCommand{},
		LightCommand{},
		LevelCommand{},
		SyncCommand{},
		HubCommand{},
	})
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
	dlog.Printf("refreshing...")
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
