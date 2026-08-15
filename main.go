package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"opencode-bench/internal/server"
)

func main() {
	home, _ := os.UserHomeDir()
	listen := flag.String("listen", "127.0.0.1:7777", "listen address")
	occfg := flag.String("opencode-config", filepath.Join(home, ".config", "opencode", "opencode.json"), "path to opencode.json")
	tasksDir := flag.String("tasks", "tasks", "task library directory")
	runsDir := flag.String("runs", "runs", "runs output directory")
	ocbin := flag.String("opencode", "opencode", "opencode binary")
	flag.Parse()

	s := server.New(server.Config{
		OpencodeConfigPath: *occfg,
		TasksDir:           *tasksDir,
		RunsDir:            *runsDir,
		OpencodeBin:        *ocbin,
	})
	log.Printf("opencode-bench listening on http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, s.Handler()))
}
