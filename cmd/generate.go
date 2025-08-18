package cmd

import (
	"embed"

	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"os/exec"

	"github.com/spf13/cobra"
)

type CustomError struct {
	Err error
}

func (e *CustomError) Error() string {
	return e.Err.Error()
}

//go:embed templates/*
var templatesFS embed.FS

// --- Prerequisite Check Function ---
func checkInitPrerequisite() error {
	const usedPortsFile = "used_ports.json"
	_, err := os.Stat(usedPortsFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("project not initialized. Please run 'gores init' first to set up the basic project structure and auth service.")
	} else if err != nil {
		return fmt.Errorf("error checking project initialization status: %w", err)
	}
	return nil
}

// --- Cobra Commands ---
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the gores project with default pkg and auth-service",
	Long:  "Creates the shared 'pkg' directory structure and generates the essential 'auth-service' by default.",
	RunE: func(cmd *cobra.Command, args []string) error {
		const (
			authServiceName       = "auth-service"
			authServicePort       = "8080"
			usedPortsFile         = "used_ports.json"
			nextAvailablePortFile = "next_available_port.txt"
		)

		fmt.Println("Initializing gores project...")

		if err := createSharedPkg(); err != nil {
			return fmt.Errorf("failed to create shared pkg: %w", err)
		}

		if ServiceExists(authServiceName) { // Call from port_management.go
			fmt.Printf("Auth service '%s' already exists, skipping generation.\n", authServiceName)
			if err := WriteUsedPortForService(8080, authServiceName, usedPortsFile); err != nil { // Call from port_management.go
				return fmt.Errorf("failed to ensure auth service port is registered: %w", err)
			}
		} else {
			fmt.Printf("Generating default auth service '%s' on port %s...\n", authServiceName, authServicePort)
			if err := createAuthMicroservice(authServiceName, authServicePort); err != nil {
				return fmt.Errorf("failed to generate auth service: %w", err)
			}
			if err := WriteUsedPortForService(8080, authServiceName, usedPortsFile); err != nil {
				return fmt.Errorf("failed to record auth service port: %w", err)
			}
		}
		currentNextPort := 8080 // This will be the base for comparison
		portBytes, err := os.ReadFile(nextAvailablePortFile)
		if err == nil {
			p, convErr := strconv.Atoi(string(portBytes))
			if convErr == nil {
				currentNextPort = p
			}
		}

		if currentNextPort <= 8080 { // If the file isn't updated or points to 8080 or less
			if err := os.WriteFile(nextAvailablePortFile, []byte(strconv.Itoa(8081)), 0644); err != nil {
				return fmt.Errorf("failed to update next available port file: %w", err)
			}
			fmt.Println("Next auto-assigned port will start from 8081.")
		} else {
			fmt.Printf("Next auto-assigned port already set to %d.\n", currentNextPort)
		}

		fmt.Println("gores project initialized successfully! 🎉")
		fmt.Println("You can now generate new microservices using: gores generate [service-name] [port(optional)]")
		fmt.Println("Or list services using: gores list-services")
		return nil
	},
}

// generateCmd is the Cobra command for generating a new microservice.
var generateCmd = &cobra.Command{
	Use:   "generate [service-name] [port]",
	Short: "Generate microservice boilerplate code",
	Long:  "Generate microservice boilerplate code including router, controller, service, entity, go.mod, Dockerfile, and go.sum.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires service name argument")
		}
		if len(args) > 1 {
			portStr := args[1]
			if len(portStr) == 0 {
				return fmt.Errorf("port cannot be empty string")
			}
			p, err := strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("port must be a valid number")
			}
			if p < 1024 || p > 65535 {
				return fmt.Errorf("port must be between 1024 and 65535")
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Prerequisite Check ---
		if err := checkInitPrerequisite(); err != nil {
			return err
		}
		// --- End Prerequisite Check ---

		serviceName := args[0]
		usedPortsFile := "used_ports.json" // File to track assigned ports
		servicesDir := "services"          // Base directory for microservices

		// 1. Check if the service directory already exists (delegated to port_management.go).
		servicePath := filepath.Join(servicesDir, serviceName)
		if _, err := os.Stat(servicePath); !os.IsNotExist(err) { // Still using os.Stat here for initial check
			return fmt.Errorf("a service with the name '%s' already exists at %s", serviceName, servicePath)
		}

		var port int
		if len(args) > 1 && args[1] != "" {
			var err error
			port, err = strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("port must be a valid number: %w", err)
			}

			// Check if the user-provided port is already used or occupied (delegated to port_management.go).
			usedPorts, err := ReadUsedPorts(usedPortsFile) // Call from port_management.go
			if err != nil {
				return fmt.Errorf("failed to read used ports file: %w", err)
			}

			if IsPortUsed(usedPorts, port) { // Call from port_management.go
				return fmt.Errorf("the port '%d' is already assigned to another service", port)
			}

			// Also check if the port is currently in use at the OS level
			addr := fmt.Sprintf(":%d", port)
			l, err := net.Listen("tcp", addr)
			if err == nil { // Port is free at OS level
				l.Close()
				// Store the user-provided port with the service name (delegated to port_management.go).
				err = WriteUsedPortForService(port, serviceName, usedPortsFile) // Call from port_management.go
				if err != nil {
					return fmt.Errorf("failed to store user-provided port: %w", err)
				}
			} else {
				return fmt.Errorf("the port '%d' is currently in use by another process on your system", port)
			}

		} else {
			// Logic for automatically finding a port (delegated to port_management.go).
			startPort := 8080                    // Starting port for auto-assignment.
			nextPortFile := "next_available_port.txt" // File to store the next suggested port

			p, err := ReadAndIncrementPortWithUsed(startPort, serviceName, nextPortFile, usedPortsFile) // Call from port_management.go
			if err != nil {
				return fmt.Errorf("failed to get next available port: %w", err)
			}
			port = p
		}

		// Generate the generic microservice (delegated to service_generation.go).
		// createSharedPkg() is implicitly handled as part of createMicroservice if needed.
		if err := createMicroservice(serviceName, strconv.Itoa(port), "templates/"); err != nil {
			return fmt.Errorf("failed to generate microservice: %w", err)
		}

		fmt.Printf("Service '%s' generated successfully on port %s.\n", serviceName, strconv.Itoa(port))
		return nil
	},
}

// listServicesCmd is the Cobra command to list all services and their ports.
var listServicesCmd = &cobra.Command{
	Use:   "list-services",
	Short: "List all generated services and their ports",
	Long:  "Displays a list of all microservices that have been generated, along with the ports they are using.",
	Run: func(cmd *cobra.Command, args []string) {
		// --- Prerequisite Check ---
		if err := checkInitPrerequisite(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error()) // Print error to stderr
			return // Exit early
		}
		// --- End Prerequisite Check ---

		usedPortsFile := "used_ports.json"

		usedPorts, err := ReadUsedPorts(usedPortsFile) // Call from port_management.go
		if err != nil {
			if os.IsNotExist(err) { // More precise check for file not found
				fmt.Println("No services have been generated yet (used_ports.json not found).")
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading used ports file: %v\n", err)
			return
		}

		if len(usedPorts.Ports) == 0 {
			fmt.Println("No services have been generated yet.")
		} else {
			fmt.Println("--- Generated Services ---")
			for _, pInfo := range usedPorts.Ports { // PortInfo struct is also implicitly known
				fmt.Printf("Service: %-25s Port: %d\n", pInfo.Service, pInfo.Port)
			}
			fmt.Println("--------------------------")
		}
	},
}

// modTidyAllCmd is the new Cobra command to run 'go mod tidy' in the pkg/ and all service directories.
var modTidyAllCmd = &cobra.Command{
	Use:   "mod-tidy-all",
	Short: "Runs 'go mod tidy' in pkg/ and all generated service directories",
	Long:  "Executes 'go mod tidy' in the 'pkg/' folder and then in each subdirectory within the 'services/' folder, ensuring all module dependencies are clean.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Running 'go mod tidy' across the monorepo...")

		originalDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		// Ensure we always return to the original directory at the end
		defer func() {
			if cerr := os.Chdir(originalDir); cerr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to change back to original directory '%s': %v\n", originalDir, cerr)
			}
		}()

		// 1. Run 'go mod tidy' in the pkg/ directory
		pkgDir := filepath.Join(originalDir, "pkg") // Construct full path to pkg/
		if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
			fmt.Printf("No 'pkg/' directory found, skipping 'go mod tidy' for pkg.\n")
		} else if err != nil {
			return fmt.Errorf("failed to check 'pkg/' directory: %w", err)
		} else {
			fmt.Printf("\nRunning 'go mod tidy' in: %s\n", pkgDir)
			if err := runGoModTidy(pkgDir); err != nil { // Run in pkgDir
				fmt.Fprintf(os.Stderr, "Error: Failed to run 'go mod tidy' in 'pkg/': %v\n", err)
				// Continue, but log the error
			}
		}

		// 2. Iterate through 'services/' directory and run 'go mod tidy' in each service
		servicesDir := "services"
		serviceFolders, err := os.ReadDir(servicesDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("No 'services/' directory found, skipping service-specific tidy.\n")
				// No error if services directory doesn't exist yet, just means nothing to tidy
				return nil
			}
			return fmt.Errorf("failed to read 'services/' directory: %w", err)
		}

		for _, entry := range serviceFolders {
			if entry.IsDir() {
				servicePath := filepath.Join(servicesDir, entry.Name())
				fmt.Printf("\nRunning 'go mod tidy' in: %s\n", servicePath)

				// Change to service directory
				if err := os.Chdir(servicePath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to change to service directory '%s': %v\n", servicePath, err)
					continue // Continue to next service
				}

				// Run go mod tidy in the service directory
				if err := runGoModTidy("."); err != nil {
					fmt.Fprintf(os.Stderr, "Error: Failed to run 'go mod tidy' in '%s': %v\n", servicePath, err)
					// Optionally, you might want to stop here or collect all errors
				}

				// Change back to original directory (handled by defer, but good practice for clarity in loop)
				if err := os.Chdir(originalDir); err != nil {
					return fmt.Errorf("failed to change back to original directory after processing '%s': %w", servicePath, err)
				}
			}
		}

		fmt.Println("\n'go mod tidy' completed across all relevant directories. ✨")
		return nil
	},
}

// runGoModTidy executes the 'go mod tidy' command in the specified directory.
func runGoModTidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir // Set the working directory for the command
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// init function to add commands to the root command.
func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(listServicesCmd)
	rootCmd.AddCommand(modTidyAllCmd)
}