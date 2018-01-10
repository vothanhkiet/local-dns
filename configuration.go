package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
)

// BindConfig bind
type BindConfig struct {
	UDP string `json:"udp"`
	TCP string `json:"tcp"`
}

// LogConfig log config
type LogConfig struct {
	Level   string `json:"level"`
	Console bool   `json:"enable_console"`
	File    bool   `json:"enable_file"`
}

// Configuration configuration
type Configuration struct {
	Bind     BindConfig   `json:"bind"`
	Resolver string       `json:"resolver"`
	TTL      int          `json:"ttl"`
	Log      LogConfig    `json:"log"`
	Hosts    []ItemConfig `json:"hosts"`
	TLDS     []ItemConfig `json:"tlds"`
	Env      string       `json:"-"`
}

// ItemConfig host
type ItemConfig struct {
	Key  string `json:"key"`
	Ipv4 string `json:"ipv4"`
	Ipv6 string `json:"ipv6"`
}

// GetCurrentEnvironment get current environment
func (c *Configuration) GetCurrentEnvironment() string {
	return c.Env
}

func (c *Configuration) init(path string) {
	c.Env = os.Getenv("GO_ENV")
	if c.Env == "" {
		fmt.Println("Warning: Load development environment due to lack of GO_ENV value.")
		c.Env = "development"
	}
	content, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("Error while reading config file", err)
	}
	jsonErr := json.Unmarshal(content, c)
	if jsonErr != nil {
		fmt.Println("Error while parsing config file", jsonErr)
	}

	if c.Env == "development" {
		c.Log.File = false
	}
	if len(c.TLDS) == 0 {
		c.TLDS = append(c.TLDS, ItemConfig{
			Key:  "dev.io",
			Ipv4: "127.0.0.1",
			Ipv6: "::1",
		})
	}
	if c.Resolver == "" {
		c.Resolver = "/etc/resolv.conf"
	}
	if c.TTL == 0 {
		c.TTL = 86400
	}
}

var instance *Configuration
var initialized int32
var mu sync.Mutex

// GetConfiguration get configuration singleton
func GetConfiguration(path string) *Configuration {
	if atomic.LoadInt32(&initialized) == 1 {
		return instance
	}
	mu.Lock()
	defer mu.Unlock()
	if initialized == 0 {
		instance = &Configuration{}
		instance.init(path)
		atomic.StoreInt32(&initialized, 1)
	}
	return instance
}
