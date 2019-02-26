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

package app

import (
	"io/ioutil"
	"path"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type SecretWatcher struct {
	basePath string
	watcher  *fsnotify.Watcher
	lock     sync.RWMutex
	pointers map[string]*string
}

func NewSecretWatcher(path string) *SecretWatcher {
	return &SecretWatcher{
		basePath: path,
		pointers: map[string]*string{},
	}
}

func (w *SecretWatcher) Start(stop <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	go func() {
		for {
			select {
			case <-stop:
				// close the watcher when shuting down
				w.watcher.Close()
				return
			case event := <-w.watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					// secret was modified
					w.updateFromFile(event.Name)
				}
			case err := <-w.watcher.Errors:
				log.Error(err, "fsnotify error")
			}
		}
	}()

	return nil
}

func (w *SecretWatcher) pathToFile(key string) string {
	return path.Join(w.basePath, key)
}

func (w *SecretWatcher) WatchFor(key string) *string {
	// initialize an empty string
	emptyStr := ""
	value := &emptyStr

	if envValue := getEnvP(key); envValue != nil {
		*value = *envValue
	}

	// get the secret file path
	path := w.pathToFile(key)

	// link secret address to file path
	w.registerPointerFor(path, value)

	// update from file the secret
	w.updateFromFile(path)

	// add file to watcher
	if err := w.watcher.Add(path); err != nil {
		log.Error(err, "failed to add secret to fsnotify", "file", path)
	}

	return value
}

// updateFromFile get the registered pointer for given file, reads the content of the file and
// writes it at that pointer
func (w *SecretWatcher) updateFromFile(file string) {
	ptr := w.getPointerFor(file)
	if content, err := ioutil.ReadFile(file); err == nil {
		*ptr = string(content)
	} else {
		log.Error(err, "fail to read the file", "file", file)
	}
}

func (w *SecretWatcher) getPointerFor(file string) *string {
	w.lock.RLock()
	defer w.lock.RUnlock()
	p, ok := w.pointers[file]
	if !ok {
		log.Info("file not registered", "file", file)
	}
	return p
}

func (w *SecretWatcher) registerPointerFor(file string, ptr *string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.pointers[file] = ptr
}
