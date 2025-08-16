package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"net"
	"strconv"
	"embed"

	"github.com/spf13/cobra"
	"encoding/json"
)

type CustomError struct {
    Err error
}

func (e *CustomError) Error() string {
    return e.Err.Error()
}

//go:embed templates/*
var templatesFS embed.FS

// PortInfo represents a single entry in the used ports file.
type PortInfo struct {
	Port    int    `json:"port"`
	Service string `json:"service"`
}

// UsedPorts represents the entire used ports file content.
type UsedPorts struct {
	Ports []PortInfo `json:"used_ports"`
}

// serviceExists checks if a directory for the service already exists.
func serviceExists(serviceName string) bool {
    // os.Stat gets file info. If the file doesn't exist, it returns a specific error.
    _, err := os.Stat(serviceName)
    return !os.IsNotExist(err)
}

// readUsedPorts reads the used ports from a JSON file.
func readUsedPorts(filename string) (*UsedPorts, error) {
	data := &UsedPorts{}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty UsedPorts
			return data, nil
		}
		return nil, err
	}

	err = json.Unmarshal(content, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// writeUsedPorts writes the used ports data to a JSON file.
func writeUsedPorts(filename string, data *UsedPorts) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, bytes, 0644)
}

// isPortUsed checks if a port is in the UsedPorts slice.
func isPortUsed(used *UsedPorts, port int) bool {
	for _, p := range used.Ports {
		if p.Port == port {
			return true
		}
	}
	return false
}

// getNextAvailablePort finds the next available port, skipping used and occupied ports.
func getNextAvailablePort(start int, used *UsedPorts) (int, error) {
	for port := start; port <= 65535; port++ {
		if isPortUsed(used, port) {
			continue
		}

		addr := fmt.Sprintf(":%d", port)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			// Port in use at OS level, skip
			continue
		}
		l.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found starting at %d", start)
}

// readAndIncrementPortWithUsed finds the next available port and stores it.
func readAndIncrementPortWithUsed(start int, serviceName, nextPortFile, usedPortsFile string) (int, error) {
	// Read next port number
	nextPort := start
	portBytes, err := ioutil.ReadFile(nextPortFile)
	if err == nil {
		p, err := strconv.Atoi(string(portBytes))
		if err == nil && p >= start {
			nextPort = p
		}
	}

	// Read used ports slice
	usedPorts, err := readUsedPorts(usedPortsFile)
	if err != nil {
		return 0, err
	}

	// Get next available port skipping used
	port, err := getNextAvailablePort(nextPort, usedPorts)
	if err != nil {
		return 0, err
	}

	// Add new port to used ports slice with the service name
	usedPorts.Ports = append(usedPorts.Ports, PortInfo{Port: port, Service: serviceName})

	// Write back updated used ports
	err = writeUsedPorts(usedPortsFile, usedPorts)
	if err != nil {
		return 0, err
	}

	// Update next port file to port+1
	err = ioutil.WriteFile(nextPortFile, []byte(strconv.Itoa(port+1)), 0644)
	if err != nil {
		return 0, err
	}

	return port, nil
}

// writeUsedPortForService adds a user-provided port and service to the used ports file.
func writeUsedPortForService(port int, serviceName, filename string) error {
	usedPorts, err := readUsedPorts(filename)
	if err != nil {
		return err
	}

	// Check if the port is already in the list
	if !isPortUsed(usedPorts, port) {
		usedPorts.Ports = append(usedPorts.Ports, PortInfo{Port: port, Service: serviceName})
		return writeUsedPorts(filename, usedPorts)
	}

	// Port is already in the list, no need to write.
	return nil
}

var generateCmd = &cobra.Command{
	Use:   "generate [service-name] [port]",
	Short: "Generate code resources",
	Long:  "Generate microservice boilerplate code including router, controller, service, entity, go.mod, Dockerfile, and go.sum.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires service name argument")
		}
		if len(args) > 1 {
			port := args[1]
			if len(port) == 0 {
				return fmt.Errorf("port cannot be empty string")
			}
			p, err := strconv.Atoi(port)
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
		serviceName := args[0]
		usedPortsFile := "used_ports.json"

		// 1. Check if the service name already exists.
		// Assuming you have a function to check for existing services.
		if serviceExists(serviceName) {
			return fmt.Errorf("a service with the name '%s' already exists", serviceName)
		}

		var port int
		if len(args) > 1 && args[1] != "" {
			var err error
			port, err = strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("port must be a valid number: %w", err)
			}
			
			// 2. Check if the user-provided port is already used.
			usedPorts, err := readUsedPorts(usedPortsFile)
			if err != nil {
				return fmt.Errorf("failed to read used ports file: %w", err)
			}
			if isPortUsed(usedPorts, port) {
				return fmt.Errorf("the port '%d' is already in use", port)
			}
			
			// Store the user-provided port with the service name.
			err = writeUsedPortForService(port, serviceName, usedPortsFile)
			if err != nil {
				return fmt.Errorf("failed to store user-provided port: %w", err)
			}

		} else {
			// Logic for automatically finding a port remains the same.
			portFile := "port.json"
			p, err := readAndIncrementPortWithUsed(8080, serviceName, portFile, usedPortsFile)
			if err != nil {
				return fmt.Errorf("failed to get next available port: %w", err)
			}
			port = p
		}

		return generateService(serviceName, strconv.Itoa(port))
	},
}

// listServicesCmd is a new Cobra command to list all services and their ports.
var listServicesCmd = &cobra.Command{
    Use:   "list-services",
    Short: "List all generated services and their ports",
    Long:  "Displays a list of all microservices that have been generated, along with the ports they are using.",
    Run: func(cmd *cobra.Command, args []string) {
        usedPortsFile := "used_ports.json"

        usedPorts, err := readUsedPorts(usedPortsFile)
        if err != nil {
            // Check if the error is due to the file not existing.
            if os.IsNotExist(err) {
                fmt.Println("No services have been generated yet")
                return
            }
            // For other file-related errors, print a more detailed message.
            fmt.Fprintf(os.Stderr, "Error reading used ports file: %v\n", err)
            return
        }

        if len(usedPorts.Ports) == 0 {
            fmt.Println("No services have been generated yet.")
        } else {
            fmt.Println("--- Generated Services ---")
            for _, pInfo := range usedPorts.Ports {
                fmt.Printf("Service: %-25s Port: %d\n", pInfo.Service, pInfo.Port)
            }
            fmt.Println("--------------------------")
        }
    },
}

func init() {
	rootCmd.AddCommand(generateCmd)

	rootCmd.AddCommand(listServicesCmd)
}

func generateService(name, port string) error {
	// 1. Setup shared pkg folder and files
	if err := createSharedPkg(); err != nil {
		return err
	}

	// 2. Setup microservice-specific folders and files
	if err := createMicroservice(name, port); err != nil {
		return err
	}

	fmt.Printf("Service '%s' generated successfully on port %s.\n", name, port)
	return nil
}

// createSharedPkg creates pkg folder with entities, database connection, http middleware, go.mod and go.sum
func createSharedPkg() error {
	pkgPath := "pkg"
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		fmt.Println("Creating shared pkg/ folder...")
		if err := os.Mkdir(pkgPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create pkg folder: %w", err)
		}
	} else {
		fmt.Println("Shared pkg/ folder already exists, skipping creation.")
	}

	// Create go.mod in pkg folder if not exists
	pkgGoModPath := filepath.Join(pkgPath, "go.mod")
	if _, err := os.Stat(pkgGoModPath); os.IsNotExist(err) {
		fmt.Println("Creating pkg/go.mod...")
		pkgGoModContent := []byte("module pkg\n\ngo 1.20\n") // customize if needed
		if err := os.WriteFile(pkgGoModPath, pkgGoModContent, 0644); err != nil {
			return fmt.Errorf("failed to create pkg/go.mod: %w", err)
		}
	} else {
		fmt.Println("pkg/go.mod already exists, skipping creation.")
	}

	// Create empty go.sum in pkg folder if not exists
	pkgGoSumPath := filepath.Join(pkgPath, "go.sum")
	if _, err := os.Stat(pkgGoSumPath); os.IsNotExist(err) {
		fmt.Println("Creating pkg/go.sum...")
		if err := os.WriteFile(pkgGoSumPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create pkg/go.sum: %w", err)
		}
	} else {
		fmt.Println("pkg/go.sum already exists, skipping creation.")
	}

	// Create pkg/entities folder
	entitiesPkgPath := filepath.Join(pkgPath, "entities")
	if _, err := os.Stat(entitiesPkgPath); os.IsNotExist(err) {
		fmt.Println("Creating pkg/entities/ folder...")
		if err := os.MkdirAll(entitiesPkgPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create pkg/entities folder: %w", err)
		}
	} else {
		fmt.Println("pkg/entities/ folder already exists, skipping creation.")
	}

	// Create pkg/database folder and connection.go from embedded template
	dbPkgPath := filepath.Join(pkgPath, "database")
	if _, err := os.Stat(dbPkgPath); os.IsNotExist(err) {
		fmt.Println("Creating pkg/database folder...")
		if err := os.MkdirAll(dbPkgPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create pkg/database folder: %w", err)
		}
	} else {
		fmt.Println("pkg/database folder already exists, skipping creation.")
	}

	dbOutputPath := filepath.Join(dbPkgPath, "connection.go")
	if _, err := os.Stat(dbOutputPath); os.IsNotExist(err) {
		dbContent, err := templatesFS.ReadFile("templates/database_connection.tmpl")
		if err != nil {
			return fmt.Errorf("failed to read database connection template: %w", err)
		}

		dbTemplate, err := template.New("database_connection.tmpl").Parse(string(dbContent))
		if err != nil {
			return fmt.Errorf("failed to parse database connection template: %w", err)
		}

		dbFile, err := os.Create(dbOutputPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", dbOutputPath, err)
		}
		defer dbFile.Close()

		err = dbTemplate.Execute(dbFile, nil) // no vars needed
		if err != nil {
			return fmt.Errorf("failed to execute database connection template: %w", err)
		}
	} else {
		fmt.Println("pkg/database/connection.go already exists, skipping creation.")
	}

	// Create pkg/http folder and middleware.go from embedded template
	httpPkgPath := filepath.Join(pkgPath, "http")
	if _, err := os.Stat(httpPkgPath); os.IsNotExist(err) {
		fmt.Println("Creating pkg/http folder...")
		if err := os.MkdirAll(httpPkgPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create pkg/http folder: %w", err)
		}
	} else {
		fmt.Println("pkg/http folder already exists, skipping creation.")
	}

	middlewareOutputPath := filepath.Join(httpPkgPath, "middleware.go")
	if _, err := os.Stat(middlewareOutputPath); os.IsNotExist(err) {
		middlewareContent, err := templatesFS.ReadFile("templates/middleware.tmpl")
		if err != nil {
			return fmt.Errorf("failed to read middleware template: %w", err)
		}

		middlewareTemplate, err := template.New("middleware.tmpl").Parse(string(middlewareContent))
		if err != nil {
			return fmt.Errorf("failed to parse middleware template: %w", err)
		}

		middlewareFile, err := os.Create(middlewareOutputPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", middlewareOutputPath, err)
		}
		defer middlewareFile.Close()

		err = middlewareTemplate.Execute(middlewareFile, nil) // no vars
		if err != nil {
			return fmt.Errorf("failed to execute middleware template: %w", err)
		}
	} else {
		fmt.Println("pkg/http/middleware.go already exists, skipping creation.")
	}

	return nil
}

// createMicroservice scaffolds microservice folders and files from embedded templates
func createMicroservice(name, port string) error {
	basePath := filepath.Join(name, "src")
	subFolders := []string{
		filepath.Join(basePath, "cmd"),
		filepath.Join(basePath, "internal"),
	}

	for _, folder := range subFolders {
		if err := os.MkdirAll(folder, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create folder %s: %w", folder, err)
		}
	}

	templates := []string{
		"main.tmpl",
		"router.tmpl",
		"controller.tmpl",
		"service.tmpl",
		"go.mod.tmpl",
		"Dockerfile.tmpl",
		"entity_pkg.tmpl", // New template for the entity
	}

	outputFiles := []string{
		filepath.Join("services" ,basePath, "cmd", "main.go"),
		filepath.Join("services" ,basePath, "internal", "router.go"),
		filepath.Join("services" ,basePath, "internal", "controller.go"),
		filepath.Join("services" ,basePath, "internal", "service.go"),
		filepath.Join("services" ,name, "go.mod"),
		filepath.Join("services",name, "Dockerfile"),
		filepath.Join("pkg", "entities", fmt.Sprintf("%s.entity.go", name)),
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"title": strings.Title,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	rootDir := filepath.Base(cwd)

	data := struct {
		Name    string
		Port    string
		RootDir string
	}{
		Name:    name,
		Port:    port,
		RootDir: rootDir,
	}

	for i, tmplFile := range templates {
		content, err := templatesFS.ReadFile("templates/" + tmplFile)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", tmplFile, err)
		}

		t, err := template.New(tmplFile).Funcs(funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", tmplFile, err)
		}

		f, err := os.Create(outputFiles[i])
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputFiles[i], err)
		}

		err = t.Execute(f, data)
		f.Close()
		if err != nil {
			return fmt.Errorf("failed to execute template %s: %w", tmplFile, err)
		}
	}

	// Create empty go.sum file (not templated)
	goSumPath := filepath.Join(basePath, "go.sum")
	if _, err := os.Stat(goSumPath); os.IsNotExist(err) {
		if err := os.WriteFile(goSumPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create go.sum: %w", err)
		}
	}

	return nil
}