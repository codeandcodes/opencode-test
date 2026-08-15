package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"opencode-bench/internal/runner"
	"opencode-bench/internal/server"
	"opencode-bench/internal/store"
)

func main() {
	home, _ := os.UserHomeDir()
	listen := flag.String("listen", "127.0.0.1:7777", "listen address")
	occfg := flag.String("opencode-config", filepath.Join(home, ".config", "opencode", "opencode.json"), "path to opencode.json")
	tasksDir := flag.String("tasks", "tasks", "task library directory")
	runsDir := flag.String("runs", "runs", "runs output directory")
	ocbin := flag.String("opencode", "opencode", "opencode binary")
	flag.Parse()

	st := store.New(*runsDir)
	run := runner.New(*ocbin, st)
	s := server.New(server.Config{
		OpencodeConfigPath: *occfg,
		TasksDir:           *tasksDir,
		RunsDir:            *runsDir,
		OpencodeBin:        *ocbin,
		Store:              st,
		Runner:             run,
	})

	// Agent processes run in their own process groups and would survive our
	// death; cancel the active batch and wait for the current job's kill to
	// land before exiting.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Print("shutting down, cancelling active batch")
		run.Cancel()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if running, _, _, _ := run.Active(); !running {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		os.Exit(0)
	}()

	log.Printf("opencode-bench listening on http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, s.Handler()))
}
