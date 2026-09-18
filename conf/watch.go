package conf

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const configWatchDebounce = 500 * time.Millisecond

// Watch watches the directory containing filePath. Editors commonly save a
// file by replacing it with a temporary file; watching the file itself would
// stop receiving events after that replacement.
func (p *Conf) Watch(filePath string, reload func()) error {
	_, err := watchConfig(filePath, reload)
	return err
}

func watchConfig(filePath string, reload func()) (*fsnotify.Watcher, error) {
	target, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve config file path: %w", err)
	}
	dir := filepath.Dir(target)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("new watcher error: %w", err)
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("watch config directory: %w", err)
	}

	go func() {
		defer watcher.Close()

		var debounce *time.Timer
		var debounceCh <-chan time.Time
		defer func() {
			if debounce != nil {
				debounce.Stop()
			}
		}()
		resetDebounce := func() {
			if debounce == nil {
				debounce = time.NewTimer(configWatchDebounce)
			} else {
				if !debounce.Stop() {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(configWatchDebounce)
			}
			debounceCh = debounce.C
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !samePath(event.Name, target) || event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
					continue
				}
				resetDebounce()
			case <-debounceCh:
				debounce = nil
				debounceCh = nil

				// Validate before signaling; only the server's reload loop may
				// replace the running configuration. Never mutate the receiver
				// here because the active core can still be using it.
				next := New()
				if err := next.LoadFromPath(target); err != nil {
					log.Printf("reload config error: %s", err)
					continue
				}
				log.Printf("config file changed, reloading...")
				if reload != nil {
					reload()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if err != nil {
					log.Printf("File watcher error: %s", err)
				}
			}
		}
	}()
	return watcher, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
