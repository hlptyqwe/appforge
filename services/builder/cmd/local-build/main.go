package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"appforge/services/builder/internal/worker"
)

func main() {
	flags := flag.NewFlagSet("appforge-local-build", flag.ExitOnError)
	taskFile := flags.String("task", "", "protocol-3 task bundle path")
	resultFile := flags.String("result", "", "build result path")
	_ = flags.Parse(os.Args[1:])
	if *taskFile == "" || *resultFile == "" {
		fmt.Fprintln(os.Stderr, "task and result are required")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := worker.ExecuteLocalTask(ctx, *taskFile, *resultFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
