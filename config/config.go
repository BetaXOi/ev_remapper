package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Event struct {
	Type  uint16 `yaml:"type,omitempty"`
	Code  uint16 `yaml:"code,omitempty"`
	Value int32  `yaml:"value,omitempty"`
}

type ButtonEvents struct {
	Desc string `yaml:"desc,omitempty"`
	Misc Event  `yaml:"misc"`
	Key  Event  `yaml:"key"`
	Sync Event  `yaml:"sync,omitempty"`
}

type Rule struct {
	Watch ButtonEvents   `yaml:"watch"`
	Mapto []ButtonEvents `yaml:"mapto"`
}

type Mapper struct {
	Name  string `yaml:"name"`
	Watch string `yaml:"watch,omitempty"`
	Write string `yaml:"write,omitempty"`
	Rules []Rule `yaml:"rules"`
}

type Config struct {
	Mappers []Mapper `yaml:"mappers"`
}

func LoadConfig(file string) (*Config, error) {
	if file == "" {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}

		name := filepath.Base(ex)
		file = filepath.Join(filepath.Dir(ex), strings.Split(name, ".")[0]+".yaml")
	}

	log.Printf("config: %s\n", file)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) String() string {
	text, err := yaml.Marshal(c)
	if err != nil {
		return err.Error()
	}

	return string(text)
}

func (c *Config) setDefault() {
	for i, mapper := range c.Mappers {
		for j, rule := range mapper.Rules {
			misc := rule.Watch.Misc
			if misc.Type == 0 {
				misc.Type = 4
			}
			if misc.Code == 0 {
				misc.Code = 4
			}
			key := rule.Watch.Key
			if key.Type == 0 {
				key.Type = 1
			}
			rule.Watch.Misc = misc
			rule.Watch.Key = key

			for k, mapto := range rule.Mapto {
				misc := mapto.Misc
				if misc.Type == 0 {
					misc.Type = 4
				}
				if misc.Code == 0 {
					misc.Code = 4
				}
				key := mapto.Key
				if key.Type == 0 {
					key.Type = 1
				}
				mapto.Misc = misc
				mapto.Key = key

				c.Mappers[i].Rules[j].Mapto[k] = mapto
			}

			c.Mappers[i].Rules[j] = rule
		}
	}

	log.Printf("config: %s\n", c)
}

// 获取jsX列表，例如 [js0, js1]
func getMatchJoysticks(name string) (joysticks []string, err error) {
	entry, err := os.ReadDir("/sys/class/input")
	if err != nil {
		return
	}

	for _, fi := range entry {
		if strings.HasPrefix(fi.Name(), "js") {
			path := path.Join("/sys/class/input", fi.Name(), "device/name")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			if name == strings.ReplaceAll(string(bytes.TrimSpace(data)), " ", "_") {
				joysticks = append(joysticks, fi.Name())
			}
		}
	}

	return
}

func getJoystickInputEventDevice(joystick string) (devices []string, err error) {
	entry, err := os.ReadDir("/dev/input/by-path")
	if err != nil {
		return
	}

	var joystickPath string
	for _, fi := range entry {
		linkTo, err := os.Readlink(path.Join("/dev/input/by-path", fi.Name()))
		if err != nil {
			continue
		}

		if path.Base(linkTo) == joystick {
			joystickPath = path.Join("/dev/input/by-path", fi.Name())
			break
		}
	}

	if joystickPath == "" {
		err = fmt.Errorf("not found")
		return
	}

	// pci-0000:02:00.0-usb-0:2.2:1.0-event-joystick -> ../event5
	// pci-0000:02:00.0-usb-0:2.2:1.0-joystick -> ../js0
	// pci-0000:02:00.0-usb-0:2.2:1.1-event -> ../event6

	joystickEventPath := strings.ReplaceAll(joystickPath, "-joystick", "-event-joystick")
	joystickOtherEventPath := strings.ReplaceAll(joystickPath, "0-joystick", "1-event")

	devices = append(devices, joystickOtherEventPath)
	devices = append(devices, joystickEventPath)

	return
}

// 当mapper没有提供watch和write设备路径时，通过name自动获取对应的设备
// TODO: 该方法不具备通用性，目前仅支持特定的设备
func (c *Config) expandMapper() error {
	var mappers []Mapper
	for _, mapper := range c.Mappers {
		if mapper.Name != "" && mapper.Watch == "" && mapper.Write == "" {
			joysticks, err := getMatchJoysticks(mapper.Name)
			log.Printf("joysticks: %s\n", joysticks)
			if err != nil {
				log.Panicln(err)
				continue
			}

			for _, joyjoystick := range joysticks {
				devices, err := getJoystickInputEventDevice(joyjoystick)
				log.Printf("devices: %s\n", devices)
				if err != nil {
					log.Panicln(err)
					continue
				}

				mappers = append(mappers, Mapper{
					Name:  mapper.Name,
					Watch: devices[0],
					Write: devices[1],
					Rules: mapper.Rules,
				})
			}
		} else {
			mappers = append(mappers, mapper)
		}
	}

	c.Mappers = mappers

	return nil
}

func (c *Config) Setup() error {
	if err := c.expandMapper(); err != nil {
		return err
	}

	c.setDefault()

	return nil
}
