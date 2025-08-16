package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"             // Changed from "io/ioutil" to "os" for file operations
	"path/filepath"  // Import filepath for ServiceExists
	"strconv"
)

// PortInfo represents a single entry in the used ports file.
type PortInfo struct {
	Port    int    `json:"port"`
	Service string `json:"service"`
}

// UsedPorts represents the entire used ports file content.
type UsedPorts struct {
	Ports []PortInfo `json:"used_ports"`
}

// ServiceExists checks if a directory for the service already exists.
// It assumes the services are directly under the 'services' directory in the root.
func ServiceExists(serviceName string) bool { // Exported
	servicePath := filepath.Join("services", serviceName) // Corrected path to services directory
	_, err := os.Stat(servicePath)
	return !os.IsNotExist(err)
}

// ReadUsedPorts reads the used ports from a JSON file.
func ReadUsedPorts(filename string) (*UsedPorts, error) { // Exported
	data := &UsedPorts{}

	content, err := os.ReadFile(filename) // Using os.ReadFile
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, fmt.Errorf("failed to read used ports file '%s': %w", filename, err)
	}

	err = json.Unmarshal(content, data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal used ports JSON from '%s': %w", filename, err)
	}
	return data, nil
}

// WriteUsedPorts writes the used ports data to a JSON file.
func WriteUsedPorts(filename string, data *UsedPorts) error { // Exported
	bytes, err := json.MarshalIndent(data, "", "    ") // 4 spaces for indent
	if err != nil {
		return fmt.Errorf("failed to marshal used ports data: %w", err)
	}
	return os.WriteFile(filename, bytes, 0644) // Using os.WriteFile
}

// IsPortUsed checks if a port is in the UsedPorts slice.
func IsPortUsed(used *UsedPorts, port int) bool { // Exported
	for _, p := range used.Ports {
		if p.Port == port {
			return true
		}
	}
	return false
}

// GetNextAvailablePort finds the next available port, skipping ports that are
// explicitly marked as used or are currently occupied at the OS level.
func GetNextAvailablePort(start int, used *UsedPorts) (int, error) { // Exported
	const maxPort = 65535
	for port := start; port <= maxPort; port++ {
		if IsPortUsed(used, port) {
			continue
		}

		addr := fmt.Sprintf(":%d", port)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		l.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found starting at %d up to %d", start, maxPort)
}

// ReadAndIncrementPortWithUsed finds the next available port, records it, and increments the next start port.
// This is used for automatic port assignment.
func ReadAndIncrementPortWithUsed(start int, serviceName, nextPortFile, usedPortsFile string) (int, error) { // Exported
	nextPort := start
	portBytes, err := os.ReadFile(nextPortFile) // Using os.ReadFile
	if err == nil {
		p, err := strconv.Atoi(string(portBytes))
		if err == nil && p >= start {
			nextPort = p
		}
	}

	usedPorts, err := ReadUsedPorts(usedPortsFile)
	if err != nil {
		return 0, err
	}

	port, err := GetNextAvailablePort(nextPort, usedPorts)
	if err != nil {
		return 0, err
	}

	usedPorts.Ports = append(usedPorts.Ports, PortInfo{Port: port, Service: serviceName})

	err = WriteUsedPorts(usedPortsFile, usedPorts)
	if err != nil {
		return 0, fmt.Errorf("failed to write updated used ports file: %w", err)
	}

	err = os.WriteFile(nextPortFile, []byte(strconv.Itoa(port+1)), 0644) // Using os.WriteFile
	if err != nil {
		return 0, fmt.Errorf("failed to update next port file: %w", err)
	}

	return port, nil
}

// WriteUsedPortForService adds a user-provided port and service to the used ports file.
// This is used when a user explicitly specifies a port.
func WriteUsedPortForService(port int, serviceName, filename string) error { // Exported
	usedPorts, err := ReadUsedPorts(filename)
	if err != nil {
		return err
	}

	if !IsPortUsed(usedPorts, port) {
		usedPorts.Ports = append(usedPorts.Ports, PortInfo{Port: port, Service: serviceName})
		return WriteUsedPorts(filename, usedPorts)
	}

	return nil
}