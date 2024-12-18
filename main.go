package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/lukaross368/k8s-cc-azure/infra"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

func main() {

	var stack auto.Stack

	projectName := "k8s-cc"
	environment := "dev"
	workDir := "./pulumiState"

	controlNodes := 2
	workerNodes := 2

	log.Printf("deploying with %v worker nodes and %v control nodes", workerNodes, controlNodes)

	ctx := context.Background()

	// Check for args

	if len(os.Args) <= 1 {
		log.Println("no argument provided. Use --create or --delete")
		os.Exit(1)
	}

	deleteRequested := false
	createRequest := false
	for _, arg := range os.Args[1:] {
		if arg == "--delete" {
			deleteRequested = true
			break
		}
		if arg == "--create" {
			createRequest = true
			break
		}
	}

	if err := os.Setenv("PULUMI_BACKEND_URL", "file://."); err != nil {
		log.Printf("Failed to set PULUMI_BACKEND_URL: %v\n", err)
		os.Exit(1)
	}

	if err := os.Setenv("PULUMI_CONFIG_PASSPHRASE", "******"); err != nil {
		log.Printf("Failed to set PULUMI_CONFIG_PASSPHRASE: %v\n", err)
		os.Exit(1)
	}

	pulumiProgram := infra.ReturnPulumiFunction()

	stack, err := auto.UpsertStackInlineSource(
		ctx,
		environment,
		projectName,
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

	if deleteRequested {
		log.Println("Running `pulumi destroy` via the Automation API...")
		_, err := stack.Destroy(ctx)
		if err != nil {
			log.Printf("Failed to run pulumi destroy: %v\n", err)
			os.Exit(1)
		}
		log.Println("Destroy succeeded!")

		err = stack.Workspace().RemoveStack(ctx, environment)
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
