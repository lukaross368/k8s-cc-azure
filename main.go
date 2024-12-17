package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	ctx := context.Background()

	// Check for `--delete` argument
	deleteRequested := false
	for _, arg := range os.Args[1:] {
		if arg == "--delete" {
			deleteRequested = true
			break
		}
	}

	// Set the local backend for self-managed state
	if err := os.Setenv("PULUMI_BACKEND_URL", "file://."); err != nil {
		fmt.Printf("Failed to set PULUMI_BACKEND_URL: %v\n", err)
		os.Exit(1)
	}

	// Set the passphrase for the stack's secrets encryption
	if err := os.Setenv("PULUMI_CONFIG_PASSPHRASE", "my-strong-password"); err != nil {
		fmt.Printf("Failed to set PULUMI_CONFIG_PASSPHRASE: %v\n", err)
		os.Exit(1)
	}

	// Define the Pulumi program inline
	pulumiProgram := func(ctx *pulumi.Context) error {
		_, err := resources.NewResourceGroup(ctx, "myResourceGroup", &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String("k8s-dev-rg"),
			Location:          pulumi.String("uksouth"),
		})
		return err
	}

	projectName := "my-azure-infra"
	stackName := "dev"
	workDir := "./pulumiState"

	// Create/update the stack with a custom working directory
	s, err := auto.UpsertStackInlineSource(
		ctx,
		stackName,
		projectName,
		pulumiProgram,
		auto.WorkDir(workDir),
	)
	if err != nil {
		fmt.Printf("Failed to create or select stack: %v\n", err)
		os.Exit(1)
	}

	if deleteRequested {
		// If --delete is requested:
		// 1. Destroy the resources
		fmt.Println("Running `pulumi destroy` via the Automation API...")
		_, err := s.Destroy(ctx)
		if err != nil {
			fmt.Printf("Failed to run pulumi destroy: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Destroy succeeded!")

		// 2. Remove the stack, which deletes the state file
		err = s.Workspace().RemoveStack(ctx, stackName)
		if err != nil {
			fmt.Printf("Failed to remove stack: %v\n", err)
			os.Exit(1)
		}

		// 3. Remove all files inside the pulumiState directory, but not the directory itself
		err = clearDirectory(workDir)
		if err != nil {
			fmt.Printf("Failed to clear pulumiState directory: %v\n", err)
		}

		fmt.Println("All resources have been removed and state cleared.")
		return
	}

	// If no --delete was requested, just do the update ("pulumi up").
	fmt.Println("Running `pulumi up` via the Automation API...")
	res, err := s.Up(ctx)
	if err != nil {
		fmt.Printf("Failed to run pulumi up: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Update succeeded!")
	for k, v := range res.Outputs {
		fmt.Printf("%s: %v\n", k, v.Value)
	}
}

// clearDirectory removes all files and subdirectories in the given directory but leaves the directory itself.
func clearDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			// Recursively clear subdirectories
			if err := clearDirectory(path); err != nil {
				return err
			}
			// After clearing subdirectory, remove it
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
