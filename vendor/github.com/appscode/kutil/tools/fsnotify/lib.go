package fsnotify

import (
	"path/filepath"
	"sync/atomic"

	"github.com/appscode/go/log"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

type Watcher struct {
	WatchDir string
	Reload   func() error

	reloadCount uint64
}

func (w *Watcher) incReloadCount(filename string) {
	atomic.AddUint64(&w.reloadCount, 1)
	log.Infof("file %s reloaded: %d", filename, atomic.LoadUint64(&w.reloadCount))
}

func (w *Watcher) Run(stopCh <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	go func() {
		<-stopCh
		defer watcher.Close()
	}()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Debugln("file watcher event: --------------------------------------", event)

				filename := filepath.Clean(event.Name)
				if filename == filepath.Join(w.WatchDir, "..data") && event.Op == fsnotify.Create {
					if err := w.Reload(); err != nil {
						log.Errorf("error[%s]: %s", filename, err)
					} else {
						w.incReloadCount(filename)
					}
				}
			case err := <-watcher.Errors:
				log.Errorln("error:", err)
			}
		}
	}()

	if err = watcher.Add(w.WatchDir); err != nil {
		return errors.Errorf("error watching dir %s. Reason: %s", w.WatchDir, err)
	}

	return nil
}
