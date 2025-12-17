package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	protoURL   = "https://raw.githubusercontent.com/traP-jp/plutus/main/specs/protobuf/cornucopia.proto"
	protoDir   = "proto"
	protoFile  = "cornucopia.proto"
	outDir     = "api/protobuf"
	moduleName = "github.com/traP-jp/plutus/api/protobuf"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Find project root
	rootDir, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		return fmt.Errorf("failed to chdir to project root: %w", err)
	}

	// 1. Download proto
	if err := downloadProto(); err != nil {
		return err
	}

	// 2. Generate
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if path, err := exec.LookPath("protoc"); err == nil {
		fmt.Printf("Using local protoc: %s\n", path)
		if err := generateLocal(path); err != nil {
			return err
		}
	} else if path, err := exec.LookPath("docker"); err == nil {
		fmt.Printf("Using Docker: %s\n", path)
		if err := generateDocker(path, rootDir); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("neither protoc nor docker found")
	}

	// 3. Setup go.mod
	return setupGoMod()
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func downloadProto() error {
	if err := os.MkdirAll(protoDir, 0755); err != nil {
		return fmt.Errorf("failed to create proto dir: %w", err)
	}

	fmt.Printf("Downloading %s...\n", protoURL)
	resp, err := http.Get(protoURL)
	if err != nil {
		return fmt.Errorf("failed to get proto: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}

	outPath := filepath.Join(protoDir, protoFile)
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func generateLocal(protocPath string) error {
	// protoc -I proto --go_out=api/protobuf --go_opt=paths=source_relative --go-grpc_out=api/protobuf --go-grpc_opt=paths=source_relative proto/cornucopia.proto
	args := []string{
		"-I", protoDir,
		"--go_out=api/protobuf", "--go_opt=paths=source_relative",
		"--go-grpc_out=api/protobuf", "--go-grpc_opt=paths=source_relative",
		filepath.Join(protoDir, protoFile),
	}
	return runCmd(protocPath, args...)
}

func generateDocker(dockerPath, rootDir string) error {
	if err := runCmd(dockerPath, "build", "-f", "Dockerfile.protoc", "-t", "cornucopia-protoc", "."); err != nil {
		return fmt.Errorf("failed to build docker image: %w", err)
	}

	internalCmd := []string{
		"protoc",
		"-I", "proto",
		"--go_out=api/protobuf", "--go_opt=paths=source_relative",
		"--go-grpc_out=api/protobuf", "--go-grpc_opt=paths=source_relative",
		"proto/cornucopia.proto",
	}

	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/src", rootDir),
		"cornucopia-protoc",
	}
	args = append(args, internalCmd...)

	return runCmd(dockerPath, args...)
}

func setupGoMod() error {
	// Check if go.mod exists
	if _, err := os.Stat(filepath.Join(outDir, "go.mod")); os.IsNotExist(err) {
		fmt.Println("Initializing go.mod...")
		cmd := exec.Command("go", "mod", "init", moduleName)
		cmd.Dir = outDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to init go module: %w", err)
		}
	}

	fmt.Println("Tidying go module...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = outDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tidy go module: %w", err)
	}
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Running: %s %s\n", name, strings.Join(args, " "))
	return cmd.Run()
}
