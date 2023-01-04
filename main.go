package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	evdev "github.com/gvalkov/golang-evdev"
	"gopkg.in/yaml.v2"
)

type Event struct {
	Type  uint16 `yaml:"type"`
	Code  uint16 `yaml:"code"`
	Value int32  `yaml:"value,omitempty"`
}

type Events struct {
	Misc Event `yaml:"misc"`
	Key  Event `yaml:"key"`
	Sync Event `yaml:"sync,omitempty"`
}

type EventMapper struct {
	Watch Events `yaml:"watch"`
	Mapto Events `yaml:"mapto"`
}
type Mapper struct {
	Name   string        `yaml:"name"`
	Watch  string        `yaml:"watch"`
	Write  string        `yaml:"write"`
	Events []EventMapper `yaml:"events"`
}

type Config struct {
	Mappers []Mapper `yaml:"mappers"`
}

func main() {
	config, err := loadConfig()
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for _, mapper := range config.Mappers {
		wg.Add(1)
		go func(m Mapper) {
			defer wg.Done()
			watchInputDevice(m)
		}(mapper)
	}
	wg.Wait()
}

func loadConfig() (*Config, error) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}

	name := filepath.Base(ex)
	cfgName := strings.Split(name, ".")[0] + ".yaml"
	cfgPath := filepath.Join(filepath.Dir(ex), cfgName)

	fmt.Printf("cfgPath: %s\n", cfgPath)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("data: %s\n", string(data))

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// 监听一组连续的input event（msic, key, sync）
type watchEventsIndex int

const (
	INDEX_MSC watchEventsIndex = iota
	INDEX_KEY
	INDEX_SYN

	INDEX_SIZE
)

type WatchEvents struct {
	Misc evdev.InputEvent
	Key  evdev.InputEvent
	Sync evdev.InputEvent
}

func watchInputDevice(mapper Mapper) error {
	watchInputDevice, err := evdev.Open(mapper.Watch)
	if err != nil {
		return err
	}
	defer watchInputDevice.File.Close()
	writeInputDevice, err := evdev.Open(mapper.Write)
	if err != nil {
		return err
	}
	defer writeInputDevice.File.Close()

	// 3个事件为一组（msic, key, sync）
	var watchEvents WatchEvents
	index := INDEX_MSC
	expect := evdev.EV_MSC
	for {
		event, err := watchInputDevice.ReadOne()
		if err != nil {
			return err
		}
		fmt.Println(event)

		if expect != int(event.Type) {
			index = INDEX_MSC
			expect = evdev.EV_MSC
		}

		switch event.Type {
		case evdev.EV_MSC:
			if index == INDEX_MSC {
				watchEvents.Misc = *event
				index = INDEX_KEY
				expect = evdev.EV_KEY
			} else {
				index = INDEX_MSC
				expect = evdev.EV_MSC
				continue
			}
		case evdev.EV_KEY:
			if index == INDEX_KEY {
				watchEvents.Key = *event
				index = INDEX_SYN
				expect = evdev.EV_SYN
			} else {
				index = INDEX_MSC
				expect = evdev.EV_MSC
				continue
			}
		case evdev.EV_SYN:
			if index == INDEX_SYN {
				watchEvents.Sync = *event
				index = INDEX_MSC
				expect = evdev.EV_MSC
			} else {
				index = INDEX_MSC
				expect = evdev.EV_MSC
				continue
			}
			// 到此处，已经按照期望连续记录了msic，key， sync 3个类型的input event了
			processMapper(writeInputDevice, mapper, watchEvents)
		default:
			continue
		}
	}
}

func eventsIsMatch(expected Events, actual WatchEvents) bool {
	// misc需要全部匹配
	if expected.Misc.Type != actual.Misc.Type ||
		expected.Misc.Code != actual.Misc.Code ||
		expected.Misc.Value != actual.Misc.Value {
		return false
	}

	// key不匹配value
	if expected.Key.Type != actual.Key.Type ||
		expected.Key.Code != actual.Key.Code {
		return false
	}

	// sync可不匹配，type code value都是0
	// if expected.Sync.Type != actual.Sync.Type ||
	// 	expected.Sync.Code != actual.Sync.Code ||
	// 	expected.Sync.Value != actual.Sync.Value {
	// 	return false
	// }

	return true
}

func getNewEvents(mapto Events, actual WatchEvents) WatchEvents {
	newEvents := actual

	// misc全部替换
	newEvents.Misc.Type = mapto.Misc.Type
	newEvents.Misc.Code = mapto.Misc.Code
	newEvents.Misc.Value = mapto.Misc.Value

	// key不替换value
	newEvents.Key.Type = mapto.Key.Type
	newEvents.Key.Code = mapto.Key.Code

	// sync可不替换
	// newEvents.Sync.Type = mapto.Sync.Type
	// newEvents.Sync.Code = mapto.Sync.Code
	// newEvents.Sync.Value = mapto.Sync.Value

	return newEvents
}

func processMapper(device *evdev.InputDevice, mapper Mapper, watchEvents WatchEvents) error {
	for _, events := range mapper.Events {
		if eventsIsMatch(events.Watch, watchEvents) {
			newEvents := getNewEvents(events.Mapto, watchEvents)
			fmt.Println("newEvents")
			fmt.Printf("%+v\n", newEvents)
			if err := device.WriteOne(&newEvents.Misc); err != nil {
				fmt.Println(err)
				return err
			}
			if err := device.WriteOne(&newEvents.Key); err != nil {
				fmt.Println(err)
				return err
			}
			if err := device.WriteOne(&newEvents.Sync); err != nil {
				fmt.Println(err)
				return err
			}

			break
		}
	}

	return nil
}
