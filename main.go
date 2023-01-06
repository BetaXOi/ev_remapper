package main

import (
	"log"
	"sync"

	"github.com/BetaXOi/ev_remapper/config"
	"github.com/BetaXOi/ev_remapper/mapper"
)

func main() {
	config, err := config.LoadConfig("")
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
			wg.Done()
		}(m)
	}

	wg.Wait()
}
