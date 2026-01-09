package config

import (
	"os"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "./config.yaml"
)

type Organizer struct {
	configPath      string
	changeListeners []func()
	changeWatcher   *fsnotify.Watcher
	lastConfig      Config
}

func NewOrganizer(configPath string) (*Organizer, error) {
	path := defaultConfigPath
	if configPath != "" {
		path = configPath
	}

	// TODO проверка существования файла

	organizer := &Organizer{}
	organizer.configPath = path

	cfg, err := organizer.Load()
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	organizer.lastConfig = cfg

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	organizer.changeWatcher = watcher

	if err = organizer.changeWatcher.Add(path); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	go organizer.watch()

	return organizer, nil
}

func (o *Organizer) Close() error {
	return o.changeWatcher.Close()
}

func (o *Organizer) AddChangeListeners(fn ...func()) {
	o.changeListeners = append(o.changeListeners, fn...)
}

func (o *Organizer) Load() (Config, error) {
	var cfg Config

	data, err := os.ReadFile(o.configPath)
	if err != nil {
		zap.L().Error(err.Error())
		return cfg, err
	}

	if err = yaml.Unmarshal(data, &cfg); err != nil {
		zap.L().Error(err.Error())
		return o.lastConfig, ErrInvalidConfig
	}

	o.lastConfig = cfg

	return cfg, nil
}

func (o *Organizer) watch() {
	for {
		select {
		case event, ok := <-o.changeWatcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				for _, listener := range o.changeListeners {
					listener()
				}
			}
		case err, ok := <-o.changeWatcher.Errors:
			if !ok {
				return
			}
			zap.L().Error(err.Error())
		}
	}
}
