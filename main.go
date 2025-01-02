package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/lukaross368/k8s-cc-azure/config"
	"github.com/lukaross368/k8s-cc-azure/infra"
	"github.com/lukaross368/k8s-cc-azure/loggers"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

var workDir string = "./pulumiState"
var configPath string = "config.yaml"
var stack auto.Stack
var ctx context.Context = context.Background()

func main() {
	if len(os.Args) <= 1 {
		log.Println("No argument provided. Use --create or --delete")
		os.Exit(1)
	}
	deleteRequest := false
	createRequest := false

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--delete":
			deleteRequest = true
		case "--create":
			createRequest = true
		case "-v":
			loggers.Verbose = true
			loggers.LogVerbose("Verbose mode enabled")
		}
	}

	config, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	} else {
		loggers.LogVerbose("Config Successfully Loaded")
	}

	if createRequest {
		loggers.LogVerbose("deploying with %v worker nodes and %v control nodes", config.Workers.Nodes, config.ControlPlane.Nodes)
	}
	loggers.LogVerbose("Using Environment: %s", config.Environment)
	loggers.LogVerbose("Using ProjectName: %s", config.ProjectName)

	// TODO: clean these up after adding azure remote backend
	if err := os.Setenv("PULUMI_BACKEND_URL", "file://."); err != nil {
		log.Printf("Failed to set PULUMI_BACKEND_URL: %v\n", err)
		os.Exit(1)
	}

	// TODO: clean these up after adding azure remote backend
	if err := os.Setenv("PULUMI_CONFIG_PASSPHRASE", "strong-pass-word"); err != nil {
		log.Printf("Failed to set PULUMI_CONFIG_PASSPHRASE: %v\n", err)
		os.Exit(1)
	}

	pulumiProgram := infra.ReturnPulumiFunction(*config)

	stack, err = auto.UpsertStackInlineSource(
		ctx,
		config.Environment,
		config.ProjectName,
		pulumiProgram,
		auto.WorkDir(workDir),
	)

	if err != nil {
		log.Printf("Failed to create or select stack: %v\n", err)
		os.Exit(1)
	}

	if createRequest {

		log.Println("Running `pulumi up` via the Automation API...")
		res, err := stack.Up(ctx)
		if err != nil {
			log.Printf("Failed to run pulumi up: %v\n", err)
			os.Exit(1)
		}

		log.Println("Update succeeded!")
		for k, v := range res.Outputs {
			log.Printf("%s: %v\n", k, v.Value)
		}
		return
	}

	if deleteRequest {
		log.Println("Running `pulumi destroy` via the Automation API...")
		_, err := stack.Destroy(ctx)
		if err != nil {
			log.Printf("Failed to run pulumi destroy: %v\n", err)
			os.Exit(1)
		}
		log.Println("Destroy succeeded!")

		err = stack.Workspace().RemoveStack(ctx, config.Environment)
		if err != nil {
			log.Printf("Failed to remove stack: %v\n", err)
			os.Exit(1)
		}

		err = clearDirectory(workDir)
		if err != nil {
			log.Printf("Failed to clear pulumiState directory: %v\n", err)
		}

		log.Println("All resources have been removed and state cleared.")
		return
	}
}

func clearDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {

			if err := clearDirectory(path); err != nil {
				return err
			}

			if err := os.Remove(path); err != nil {
				return err
			}
		} else {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}
