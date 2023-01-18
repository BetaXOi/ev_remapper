package config

import (
	"bytes"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/deniswernert/udev"
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

func (m *Mapper) IsTemplate() bool {
	return m.Name != "" && m.Watch == "" && m.Write == ""
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

			if name == string(bytes.TrimSpace(data)) {
				joysticks = append(joysticks, fi.Name())
			}
		}
	}

	return
}

// 获取eventX列表，例如 [event0, event1]
func getJoystickInputEventDevice(joystick string) (devices []string, err error) {
	jsXName, err := os.ReadFile(path.Join("/sys/class/input", joystick, "device/name"))
	if err != nil {
		return
	}
	jsXName = bytes.TrimSpace(jsXName)
	jsXConsumerControl := append(bytes.TrimSpace(jsXName), []byte(" Consumer Control")...)
	jsXPhys, err := os.ReadFile(path.Join("/sys/class/input", joystick, "device/phys"))
	if err != nil {
		return
	}
	jsXPhys = bytes.TrimSpace(jsXPhys)
	jsXUSB := bytes.Split(jsXPhys, []byte("/"))[0]

	var jsXEventDevice string
	var jsXConsumerControlDevice string

	entry, err := os.ReadDir("/sys/class/input")
	if err != nil {
		return
	}
	for _, fi := range entry {
		eventX := fi.Name()
		if !strings.HasPrefix(eventX, "event") {
			continue
		}

		name, err := os.ReadFile(path.Join("/sys/class/input", eventX, "device/name"))
		if err != nil {
			continue
		}
		name = bytes.TrimSpace(name)
		phys, err := os.ReadFile(path.Join("/sys/class/input", eventX, "device/phys"))
		if err != nil {
			continue
		}
		phys = bytes.TrimSpace(phys)
		usb := bytes.Split(phys, []byte("/"))[0]

		if bytes.Equal(name, jsXName) && bytes.Equal(phys, jsXPhys) {
			jsXEventDevice = eventX
			continue
		}

		if bytes.Equal(name, jsXConsumerControl) && bytes.Equal(usb, jsXUSB) {
			jsXConsumerControlDevice = eventX
			continue
		}
	}

	devices = append(devices, jsXConsumerControlDevice)
	devices = append(devices, jsXEventDevice)

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
				log.Println(err)
				continue
			}

			for _, joyjoystick := range joysticks {
				devices, err := getJoystickInputEventDevice(joyjoystick)
				log.Printf("devices: %s\n", devices)
				if err != nil {
					log.Println(err)
					continue
				}

				mappers = append(mappers, Mapper{
					Name:  mapper.Name,
					Watch: path.Join("/dev/input", devices[0]),
					Write: path.Join("/dev/input", devices[1]),
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

// 监听uevent，当发现有新增的jsX input设备后，将设备名写入channel
func (c *Config) WatchNewJoystick(jsX chan string) (chan bool, error) {
	monitor, err := udev.NewMonitor()
	if err != nil {
		return nil, err
	}
	defer monitor.Close()

	closeCh := make(chan bool)

	events := make(chan *udev.UEvent)
	shutdown := monitor.Monitor(events)
	for {
		select {
		case c := <-closeCh:
			if c {
				shutdown <- c
			}
		case event := <-events:
			if event.Action == "add" {
				devName := path.Base(event.Env["DEVNAME"])
				subsystem := event.Env["SUBSYSTEM"]

				if strings.HasPrefix(devName, "js") && subsystem == "input" {
					log.Println("send js to channel")
					jsX <- devName
					log.Println("send done")
				}
			}
		}
	}
}

// 得到jsX设备的名称
func getJoystickName(jsX string) (string, error) {
	path := path.Join("/sys/class/input", jsX, "device/name")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(data)), nil
}

// 为jsX设备生成相应的mapper规则
func (c *Config) GenerateMapperForJoystick(jsX string) (*Mapper, error) {
	name, err := getJoystickName(jsX)
	if err != nil {
		return nil, err
	}

	devices, err := getJoystickInputEventDevice(jsX)
	log.Printf("devices: %s\n", devices)
	if err != nil {
		return nil, err
	}

	mapper := &Mapper{}
	for _, m := range c.Mappers {
		if !m.IsTemplate() {
			continue
		}
		if m.Name == name {
			mapper = &Mapper{
				Name:  name,
				Watch: path.Join("/dev/input", devices[0]),
				Write: path.Join("/dev/input", devices[1]),
				Rules: m.Rules,
			}
			break
		}
	}

	return mapper, nil
}
