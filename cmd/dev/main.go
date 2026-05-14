// Dev watcher. Rebuilds and restarts the daemon when any source file
// changes. Self-contained: no system dependency, just fsnotify + go.
//
// Usage:
//
//	coily exec dev
//
// Forwards the same flags to ./personal-dashboard as `coily exec run`
// (default --addr=127.0.0.1:31337). On rebuild failure, prints the
// error and keeps watching; the previous daemon stays alive so a
// broken save does not take the page down.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	binary       = "./personal-dashboard"
	buildPackage = "./cmd/personal-dashboard"
	debounce     = 200 * time.Millisecond
)

var watchExtensions = map[string]bool{
	".go":            true,
	".html.tmpl":     true,
	".tmpl":          true,
	".css":           true,
	".js":            true,
	".webmanifest":   true,
	".svg":           true,
}

// skipDirs are paths under the repo root the watcher should never recurse
// into. The compiled binary lives at the repo root and would otherwise
// re-trigger the watcher on every rebuild.
var skipDirs = map[string]bool{
	".git":   true,
	"dist":   true,
	"vendor": true,
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("fsnotify: %v", err)
	}
	defer w.Close()

	if err := walkAndWatch(w, root); err != nil {
		log.Fatalf("watch setup: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var mu sync.Mutex
	var current *exec.Cmd

	restart := func() {
		mu.Lock()
		defer mu.Unlock()

		if current != nil && current.Process != nil {
			_ = current.Process.Signal(syscall.SIGTERM)
			_ = current.Wait()
		}

		log.Println("dev: building...")
		out, err := exec.Command("go", "build", "-o", binary, buildPackage).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "dev: build failed:\n%s\n", out)
			return
		}

		cmd := exec.Command(binary, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "dev: start failed: %v\n", err)
			return
		}
		current = cmd
		log.Printf("dev: daemon pid=%d", cmd.Process.Pid)
	}

	restart()

	var debounceTimer *time.Timer
	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			if current != nil && current.Process != nil {
				_ = current.Process.Signal(syscall.SIGTERM)
				_ = current.Wait()
			}
			mu.Unlock()
			log.Println("dev: bye")
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if !shouldRebuildFor(ev.Name) {
				continue
			}
			// Add new directories to the watch set as they appear.
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = walkAndWatch(w, ev.Name)
				}
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounce, func() {
				log.Printf("dev: change %s", ev.Name)
				restart()
			})
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("dev: watcher err: %v", err)
		}
	}
}

func walkAndWatch(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if skipDirs[info.Name()] {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

func shouldRebuildFor(path string) bool {
	if strings.HasSuffix(path, binary) {
		return false
	}
	// Compound extensions like .html.tmpl
	for ext := range watchExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
