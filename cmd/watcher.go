package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const ignorePattern = `\.swp$|~$|^\.DS_Store$|^4913$`
const lockTime = 100 * time.Millisecond

func createWatcher(dir string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return watcher, err
	}
	logInfo("Watching %s/ for changes", dir)
	err = watcher.Add(dir)
	return watcher, err
}

// createRecursiveWatcher watches dir and all subdirectories for changes.
func createRecursiveWatcher(dir string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return watcher, err
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			if info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			logInfo("Watching %s/ for changes", path)
			return watcher.Add(path)
		}
		return nil
	})

	return watcher, err
}

func watch(done <-chan interface{}, errorChan chan<- error, reload chan<- bool, watcher *fsnotify.Watcher) {
	isLocked := false
	for {
		select {
		case event := <-watcher.Events:
			if isLocked {
				break
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				r := regexp.MustCompile(ignorePattern)
				if r.MatchString(event.Name) {
					logDebug("Debug [ignore]: `%s`", event.Name)
				} else {
					logInfo("Change detected in %s, refreshing", event.Name)
					isLocked = true
					reload <- true
					timer := time.NewTimer(lockTime)
					go func() {
						<-timer.C
						isLocked = false
					}()
				}
			}
		case err := <-watcher.Errors:
			errorChan <- err
		case <-done:
			return
		}
	}
}
