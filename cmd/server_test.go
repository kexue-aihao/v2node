package cmd

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wyx2685/v2node/conf"
	"github.com/wyx2685/v2node/core"
	"github.com/wyx2685/v2node/limiter"
	"github.com/wyx2685/v2node/node"
)

func TestReloadPreparationFailureKeepsCoreRunning(t *testing.T) {
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "panel temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer panel.Close()
	for name, data := range map[string]string{
		"invalid JSON":      `{"Nodes":`,
		"panel unavailable": fmt.Sprintf(`{"Nodes":[{"ApiHost":%q,"NodeID":1,"ApiKey":"test","RetryCount":0}]}`, panel.URL),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			oldConf := conf.New()
			oldCore := core.New(oldConf)
			if err := oldCore.Start(nil); err != nil {
				t.Fatal(err)
			}
			defer oldCore.Close()
			oldNodes := &node.Node{}
			currentNodes, currentCore := oldNodes, oldCore
			err := reload(path, &currentNodes, &currentCore)
			if !errors.Is(err, errReloadPreparation) {
				t.Fatalf("expected recoverable preparation error, got %v", err)
			}
			if currentNodes != oldNodes || currentCore != oldCore || oldCore.Config != oldConf || !oldCore.Server.IsRunning() {
				t.Fatal("failed preparation changed or stopped the running instance")
			}
		})
	}
}

func TestReloadAddsPanelNode(t *testing.T) {
	ports := make(map[int]int)
	for _, id := range []int{1, 2} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ports[id] = listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
	}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/server/config":
			id, _ := strconv.Atoi(r.URL.Query().Get("node_id"))
			fmt.Fprintf(w, `{"protocol":"vless","listen_ip":"127.0.0.1","server_port":%d,"base_config":{"push_interval":3600,"pull_interval":3600}}`, ports[id])
		case "/api/v1/server/UniProxy/user":
			fmt.Fprint(w, `{"users":[{"id":1,"uuid":"00000000-0000-4000-8000-000000000001"}]}`)
		case "/api/v1/server/UniProxy/alivelist":
			fmt.Fprint(w, `{"alive":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer panel.Close()
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig := func(ids ...int) {
		t.Helper()
		var entries []string
		for _, id := range ids {
			entries = append(entries, fmt.Sprintf(`{"ApiHost":%q,"NodeID":%d,"ApiKey":"test"}`, panel.URL, id))
		}
		if err := os.WriteFile(path, []byte(`{"Nodes":[`+strings.Join(entries, ",")+`]}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig(1)
	c := conf.New()
	if err := c.LoadFromPath(path); err != nil {
		t.Fatal(err)
	}
	limiter.Init()
	nodes, err := node.New(c.NodeConfigs)
	if err != nil {
		t.Fatal(err)
	}
	v2core := core.New(c)
	reloadCh := make(chan struct{}, 1)
	v2core.ReloadCh = reloadCh
	if err := v2core.Start(nodes.NodeInfos); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = nodes.Close()
		_ = v2core.Close()
	}()
	if err := nodes.Start(c.NodeConfigs, v2core); err != nil {
		t.Fatal(err)
	}
	writeConfig(1, 2)
	if err := reload(path, &nodes, &v2core); err != nil {
		t.Fatal(err)
	}
	if len(nodes.NodeInfos) != 2 || v2core.ReloadCh != reloadCh {
		t.Fatal("new node or subsequent reload channel is missing")
	}
	for _, id := range []int{1, 2} {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ports[id]), time.Second)
		if err != nil {
			t.Fatalf("node %d is not listening after reload: %v", id, err)
		}
		_ = conn.Close()
	}
}
