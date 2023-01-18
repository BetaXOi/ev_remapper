package mapper

import (
	"log"

	evdev "github.com/gvalkov/golang-evdev"
	"gopkg.in/yaml.v2"

	"github.com/BetaXOi/ev_remapper/config"
)

// 一个按键按下或释放对应一组连续的input event（msic, key, sync）
type watchEventsIndex int

const (
	INDEX_MSC watchEventsIndex = iota
	INDEX_KEY
	INDEX_SYN

	INDEX_SIZE
)

type ButtonEvents struct {
	Misc evdev.InputEvent
	Key  evdev.InputEvent
	Sync evdev.InputEvent
}

type Mapper struct {
	// 监听按键事件的设备
	watchInputDevice *evdev.InputDevice
	// 模拟按键事件源的设备
	writeInputDevice *evdev.InputDevice

	cfgMapper *config.Mapper
}

func (m *Mapper) String() string {
	text, err := yaml.Marshal(m)
	if err != nil {
		return err.Error()
	}

	return string(text)
}

func (m *Mapper) Close() {
	m.watchInputDevice.File.Close()
	m.writeInputDevice.File.Close()
}

func Init(cfgMapper *config.Mapper) (*Mapper, error) {
	mapper := &Mapper{
		cfgMapper: cfgMapper,
	}

	device, err := evdev.Open(cfgMapper.Watch)
	if err != nil {
		return nil, err
	}
	mapper.watchInputDevice = device
	log.Printf("%+v\n", device)

	device, err = evdev.Open(cfgMapper.Write)
	if err != nil {
		return nil, err
	}
	mapper.writeInputDevice = device
	log.Printf("%+v\n", device)

	return mapper, nil
}

// 监听期望的按键事件
func (m *Mapper) WatchButtonEvents() error {
	// 3个事件为一组（msic, key, sync）
	var watchedEvents ButtonEvents
	index := INDEX_MSC
	expect := evdev.EV_MSC
	for {
		event, err := m.watchInputDevice.ReadOne()
		if err != nil {
			return err
		}
		log.Println(event)

		if expect != int(event.Type) {
			index = INDEX_MSC
			expect = evdev.EV_MSC
		}

		switch event.Type {
		case evdev.EV_MSC:
			if index == INDEX_MSC {
				watchedEvents.Misc = *event
				index = INDEX_KEY
				expect = evdev.EV_KEY
			} else {
				index = INDEX_MSC
				expect = evdev.EV_MSC
				continue
			}
		case evdev.EV_KEY:
			if index == INDEX_KEY {
				watchedEvents.Key = *event
				index = INDEX_SYN
				expect = evdev.EV_SYN
			} else {
				index = INDEX_MSC
				expect = evdev.EV_MSC
				continue
			}
		case evdev.EV_SYN:
			if index == INDEX_SYN {
				watchedEvents.Sync = *event
				index = INDEX_MSC
				expect = evdev.EV_MSC
			} else {
				index = INDEX_MSC
				expect = evdev.EV_MSC
				continue
			}
			// 到此处，已经按照期望连续记录了msic，key， sync 3个类型的input event了
			newEvents, err := m.GenerateMappedButtonEvents(watchedEvents)
			if err != nil {
				log.Println(err)
				continue
			}
			if err := m.WriteButtonEvents(newEvents); err != nil {
				log.Println(err)
				return err
			}
		default:
			continue
		}
	}
}

func (event *ButtonEvents) Match(expect config.ButtonEvents) bool {
	// misc需要全部匹配
	if event.Misc.Type != expect.Misc.Type ||
		event.Misc.Code != expect.Misc.Code ||
		event.Misc.Value != expect.Misc.Value {
		return false
	}

	// key不匹配value
	if event.Key.Type != expect.Key.Type ||
		event.Key.Code != expect.Key.Code {
		return false
	}

	// sync可不匹配，type code value都是0
	// if event.Sync.Type != expect.Sync.Type ||
	// 	event.Sync.Code != expect.Sync.Code ||
	// 	event.Sync.Value != expect.Sync.Value {
	// 	return false
	// }

	return true
}

func (event *ButtonEvents) GenerateNewEvents(mapto []config.ButtonEvents) []ButtonEvents {
	newEvents := make([]ButtonEvents, len(mapto))

	for i, to := range mapto {
		newEvents[i] = *event

		// misc全部替换
		newEvents[i].Misc.Type = to.Misc.Type
		newEvents[i].Misc.Code = to.Misc.Code
		newEvents[i].Misc.Value = to.Misc.Value

		// key不替换value
		newEvents[i].Key.Type = to.Key.Type
		newEvents[i].Key.Code = to.Key.Code

		// sync可不替换
		// newEvents[i].Sync.Type = to.Sync.Type
		// newEvents[i].Sync.Code = to.Sync.Code
		// newEvents[i].Sync.Value = to.Sync.Value
	}

	return newEvents
}

// 根据映射规则，生成
func (m *Mapper) GenerateMappedButtonEvents(watchedEvents ButtonEvents) (newEvents []ButtonEvents, err error) {
	for _, rule := range m.cfgMapper.Rules {
		if !watchedEvents.Match(rule.Watch) {
			continue
		}

		newEvents = watchedEvents.GenerateNewEvents(rule.Mapto)
		break
	}
	return
}

// 写入期望的按键事件，即模拟按键对应的事件
func (m *Mapper) WriteButtonEvents(buttonEvents []ButtonEvents) (err error) {
	log.Printf("write new events: %+v\n", buttonEvents)

	for _, events := range buttonEvents {
		if err = m.writeInputDevice.WriteOne(&events.Misc); err != nil {
			return
		}
		if err = m.writeInputDevice.WriteOne(&events.Key); err != nil {
			return
		}
		if err = m.writeInputDevice.WriteOne(&events.Sync); err != nil {
			return
		}
	}

	return
}
