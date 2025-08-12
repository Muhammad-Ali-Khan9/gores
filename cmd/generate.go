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

//go:embed templates/*
var templatesFS embed.FS

type PortSequence struct {
	LastPort int `json:"last_port"`
}

type UsedPorts struct {
	Ports []int `json:"used_ports"`
}

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

func writeUsedPorts(filename string, data *UsedPorts) error {
    bytes, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(filename, bytes, 0644)
}

// Check if port is in UsedPorts.Ports slice
func isPortUsed(used *UsedPorts, port int) bool {
    for _, p := range used.Ports {
        if p == port {
            return true
        }
    }
    return false
}

// Find next available port, skipping used ports and occupied OS ports
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

func readAndIncrementPortWithUsed(start int, nextPortFile, usedPortsFile string) (int, error) {
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

    // Add new port to used ports slice
    usedPorts.Ports = append(usedPorts.Ports, port)

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

        // Choose API type
        fmt.Println("Select API type to generate:")
        fmt.Println("1) Restful")
        fmt.Println("2) GraphQL")
        fmt.Println("3) gRPC")
        fmt.Print("Enter choice (1-3): ")

        var choice int
        _, err := fmt.Scan(&choice)
        if err != nil {
            return fmt.Errorf("failed to read input: %w", err)
        }

        var port string
        if len(args) > 1 && args[1] != "" {
            port = args[1]
        } else {
            portFile := "port.json"
            usedPortsFile := "used_ports.json"

            p, err := readAndIncrementPortWithUsed(8080, portFile, usedPortsFile)
            if err != nil {
                return fmt.Errorf("failed to get next available port: %w", err)
            }
            port = strconv.Itoa(p)
        }

        switch choice {
        case 1:
            return generateService(serviceName, port) // Your Restful generator
        case 2:
            fmt.Println("GraphQL generation not implemented yet.")
            return nil
        case 3:
            fmt.Println("gRPC generation not implemented yet.")
            return nil
        default:
            return fmt.Errorf("invalid choice")
        }
    },
}

func init() {
	rootCmd.AddCommand(generateCmd)
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
		"entity_pkg.tmpl",
		"go.mod.tmpl",
		"Dockerfile.tmpl",
		"entity_pkg.tmpl",
	}

	outputFiles := []string{
		filepath.Join(basePath, "cmd", "main.go"),
		filepath.Join(basePath, "internal", "router.go"),
		filepath.Join(basePath, "internal", "controller.go"),
		filepath.Join(basePath, "internal", "service.go"),
		filepath.Join(basePath, "internal", "entity.go"),
		filepath.Join(basePath, "go.mod"),
		filepath.Join(basePath, "Dockerfile"),
		filepath.Join("pkg", "entities", strings.ToLower(name)+"_entity.go"),
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