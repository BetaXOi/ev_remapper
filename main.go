package main

import (
	"flag"
	"log"
	"sync"
	"time"

	"github.com/BetaXOi/ev_remapper/config"
	"github.com/BetaXOi/ev_remapper/mapper"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "", "config file path")
	flag.Parse()

	config, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalln(err)
	}

	if err := config.Setup(); err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	for _, cfgMapper := range config.Mappers {
		m, err := mapper.Init(&cfgMapper)
		if err != nil {
			log.Println(err)
			continue
		}

		wg.Add(1)
		go func(m *mapper.Mapper) {
			if err := m.WatchButtonEvents(); err != nil {
				log.Println(err)
			}
			m.Close()
			wg.Done()
		}(m)
	}

	jsX := make(chan string)
	go func() {
		closeCh, err := config.WatchNewJoystick(jsX)
		if err != nil {
			log.Println(err)
			return
		}
		closeCh <- true
	}()

	go func() {
		for {
			select {
			case jsx := <-jsX:
				time.Sleep(time.Second)
				cfgMapper, err := config.GenerateMapperForJoystick(jsx)
				if err != nil {
					continue
				}
				m, err := mapper.Init(cfgMapper)
				if err != nil {
					continue
				}

				go func(m *mapper.Mapper) {
					if err := m.WatchButtonEvents(); err != nil {
						log.Println(err)
					}
					m.Close()
					wg.Done()
				}(m)
			}
		}
	}()

	wg.Wait()
}
