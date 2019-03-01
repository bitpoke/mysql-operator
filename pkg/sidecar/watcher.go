/*
Copyright 2019 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sidecar

import (
	"io/ioutil"
	"sync"
	"time"
)

// UpdatableString is the interface that expose a getter for a value and an update method
type UpdatableString interface {
	// Update query for a new information and then returns true if the value changed from last
	// update, false otherwise
	Update() bool

	// returns the current value
	String() string
}

type valueFromFile struct {
	filename string

	currentValue string
}

func (vff *valueFromFile) Update() bool {
	changed := false
	if content, err := ioutil.ReadFile(vff.filename); err == nil {
		if current := string(content); vff.currentValue != current {
			changed = true
			vff.currentValue = current
		}
	} else {
		log.Error(err, "fail to read the file", "file", vff.filename)
	}

	return changed
}

func (vff *valueFromFile) String() string {
	return vff.currentValue
}

// GetValueFromFile returns a UpdatebleValue that gets it's value from provided filename
func GetValueFromFile(filename string) UpdatableString {
	return &valueFromFile{
		filename: filename,
	}
}

// updateAll given UpdatableString and returns true if any of them has updated
func updateAll(vars ...UpdatableString) bool {
	changed := false
	for _, v := range vars {
		changed = changed || v.Update()
	}

	return changed
}

// Observer is a function that is called (notified) by the reconcieLoop (Subject)
type Observer func(cfg *Config) error

// Subject the interface to handle Observers
type Subject interface {
	Start(stop <-chan struct{})
	AddObserver(string, Observer)
}

type reconcileLoop struct {
	cfg *Config

	observers *sync.Map
}

// NewSubject returns an object that implements Subject interface
func NewSubject(cfg *Config) Subject {
	return &reconcileLoop{
		cfg:       cfg,
		observers: &sync.Map{},
	}
}

func (rl *reconcileLoop) Start(stop <-chan struct{}) {
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(rl.cfg.ReconcileTime):
				rl.notifyObservers()
			}
		}
	}()
}

func (rl *reconcileLoop) notifyObservers() {
	rl.observers.Range(func(nameArg interface{}, obsArg interface{}) bool {
		name := nameArg.(string)
		obs := obsArg.(Observer)
		// call the observer
		rl.notifyObserver(name, obs)
		return true
	})
}

func (rl *reconcileLoop) notifyObserver(name string, obs Observer) {
	log.V(1).Info("notifying observer", "name", name)
	if err := obs(rl.cfg); err != nil {
		log.Error(err, "observer faild to execute", "name", name)
	}
}

func (rl *reconcileLoop) AddObserver(name string, obs Observer) {
	rl.observers.Store(name, obs)

	rl.notifyObserver(name, obs)
	log.Info("successfuly added observer", "name", name)
}
