package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeWatchConfig(t *testing.T, path string, id int) {
	t.Helper()
	data := fmt.Sprintf(`{"Nodes":[{"ApiHost":"https://panel.example","NodeID":%d,"ApiKey":"test"}]}`, id)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func startConfigWatch(t *testing.T) (string, <-chan int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	writeWatchConfig(t, path, 1)
	changes := make(chan int, 16)
	watcher, err := watchConfig(path, func() {
		c := New()
		if err := c.LoadFromPath(path); err != nil || len(c.NodeConfigs) != 1 {
			changes <- -1
			return
		}
		changes <- c.NodeConfigs[0].NodeID
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watcher.Close() })
	return path, changes
}

func expectConfigChange(t *testing.T, changes <-chan int, want int) {
	t.Helper()
	select {
	case got := <-changes:
		if got != want {
			t.Fatalf("reloaded node ID = %d, want %d", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("save of node ID %d did not trigger reload", want)
	}
}

func expectNoConfigChange(t *testing.T, changes <-chan int) {
	t.Helper()
	select {
	case id := <-changes:
		t.Fatalf("unexpected reload of node ID %d", id)
	case <-time.After(time.Second):
	}
}

func TestWatchRepeatedSaves(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("replace=%t", replace), func(t *testing.T) {
			path, changes := startConfigWatch(t)
			// Both saves finish inside the old ten-second suppression window.
			for _, id := range []int{2, 3} {
				if replace {
					temp := path + ".tmp"
					writeWatchConfig(t, temp, id)
					if err := os.Rename(temp, path); err != nil {
						t.Fatal(err)
					}
				} else {
					writeWatchConfig(t, path, id)
				}
				expectConfigChange(t, changes, id)
			}
		})
	}
}

func TestWatchDebouncesPartialWrites(t *testing.T) {
	path, changes := startConfigWatch(t)
	if err := os.WriteFile(path, []byte(`{"Nodes":`), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	writeWatchConfig(t, path, 2)
	time.Sleep(100 * time.Millisecond)
	writeWatchConfig(t, path, 3)
	expectConfigChange(t, changes, 3)
	expectNoConfigChange(t, changes)
}

func TestWatchIgnoresInvalidAndUnrelatedFiles(t *testing.T) {
	path, changes := startConfigWatch(t)
	writeWatchConfig(t, path+".swp", 2)
	expectNoConfigChange(t, changes)
	if err := os.WriteFile(path, []byte(`{"Nodes":`), 0600); err != nil {
		t.Fatal(err)
	}
	expectNoConfigChange(t, changes)
	writeWatchConfig(t, path, 3)
	expectConfigChange(t, changes, 3)
}

func TestWatchSurvivesRemovalAndRecreation(t *testing.T) {
	path, changes := startConfigWatch(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	expectNoConfigChange(t, changes)
	writeWatchConfig(t, path, 2)
	expectConfigChange(t, changes, 2)
}
