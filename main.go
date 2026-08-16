package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"opencode-bench/internal/runner"
	"opencode-bench/internal/server"
	"opencode-bench/internal/store"
)

func extraModelList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, m := range strings.Split(s, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func main() {
	home, _ := os.UserHomeDir()
	listen := flag.String("listen", "127.0.0.1:7777", "listen address")
	occfg := flag.String("opencode-config", filepath.Join(home, ".config", "opencode", "opencode.json"), "path to opencode.json")
	tasksDir := flag.String("tasks", "tasks", "task library directory")
	runsDir := flag.String("runs", "runs", "runs output directory")
	ocbin := flag.String("opencode", "opencode", "opencode binary")
	extraModels := flag.String("extra-models", "", "comma-separated provider-qualified reference models, e.g. opencode/x=Name")
	lsCfg := flag.String("llama-swap-config", "", "path to llama-swap YAML config; when set, each run snapshots the model's serving entry into its provenance")
	idleMin := flag.Int("idle-timeout", 10, "kill a job when its event stream is silent this many minutes (0 disables)")
	flag.Parse()

	st := store.New(*runsDir)
	run := runner.New(*ocbin, st)
	run.LlamaSwapConfig = *lsCfg
	run.IdleTimeout = time.Duration(*idleMin) * time.Minute
	stateFile := filepath.Join(*runsDir, ".active-batch.json")
	run.StateFile = stateFile
	s := server.New(server.Config{
		OpencodeConfigPath: *occfg,
		TasksDir:           *tasksDir,
		RunsDir:            *runsDir,
		OpencodeBin:        *ocbin,
		BatchStateFile:     stateFile,
		ExtraModels:        extraModelList(*extraModels),
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
		log.Print("shutting down, cancelling active batch (state preserved for resume)")
		run.Shutdown()
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
