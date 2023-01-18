module github.com/BetaXOi/ev_remapper

go 1.19

require (
	github.com/deniswernert/udev v0.0.0-20170418162847-a12666f7b5a1
	github.com/gvalkov/golang-evdev v0.0.0-20220815104727-7e27d6ce89b6
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/gvalkov/golang-evdev => github.com/BetaXOi/golang-evdev v0.0.0-20230104112912-901f1f6f1a15
