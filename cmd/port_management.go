package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
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
// It assumes the services are directly under the root directory where this command is run.
func ServiceExists(serviceName string) bool {
	_, err := os.Stat(serviceName)
	return !os.IsNotExist(err)
}

// ReadUsedPorts reads the used ports from a JSON file.
func ReadUsedPorts(filename string) (*UsedPorts, error) {
	data := &UsedPorts{}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty UsedPorts
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
func WriteUsedPorts(filename string, data *UsedPorts) error {
	bytes, err := json.MarshalIndent(data, "", "    ") // 4 spaces for indent
	if err != nil {
		return fmt.Errorf("failed to marshal used ports data: %w", err)
	}
	return ioutil.WriteFile(filename, bytes, 0644) // 0644 means read/write for owner, read-only for others
}

// IsPortUsed checks if a port is in the UsedPorts slice.
func IsPortUsed(used *UsedPorts, port int) bool {
	for _, p := range used.Ports {
		if p.Port == port {
			return true
		}
	}
	return false
}

// GetNextAvailablePort finds the next available port, skipping ports that are
// explicitly marked as used or are currently occupied at the OS level.
func GetNextAvailablePort(start int, used *UsedPorts) (int, error) {
	const maxPort = 65535 // Max possible TCP/UDP port number
	for port := start; port <= maxPort; port++ {
		if IsPortUsed(used, port) {
			continue // Skip if already recorded as used
		}

		// Check if the port is actually free at the OS level
		addr := fmt.Sprintf(":%d", port)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			// Port in use at OS level, skip and try next
			continue
		}
		l.Close() // Close the listener immediately as we just wanted to check availability
		return port, nil // Found an available port
	}
	return 0, fmt.Errorf("no available port found starting at %d up to %d", start, maxPort)
}

// ReadAndIncrementPortWithUsed finds the next available port, records it, and increments the next start port.
// This is used for automatic port assignment.
func ReadAndIncrementPortWithUsed(start int, serviceName, nextPortFile, usedPortsFile string) (int, error) {
	// Read next port number from file, default to 'start'
	nextPort := start
	portBytes, err := ioutil.ReadFile(nextPortFile)
	if err == nil {
		p, err := strconv.Atoi(string(portBytes))
		if err == nil && p >= start { // Ensure parsed port is not less than the initial start value
			nextPort = p
		}
	}

	// Read current list of used ports
	usedPorts, err := ReadUsedPorts(usedPortsFile)
	if err != nil {
		return 0, err
	}

	// Find the next truly available port (not in usedPorts list and not currently occupied by OS)
	port, err := GetNextAvailablePort(nextPort, usedPorts)
	if err != nil {
		return 0, err
	}

	// Add the newly assigned port to the list of used ports
	usedPorts.Ports = append(usedPorts.Ports, PortInfo{Port: port, Service: serviceName})

	// Write the updated list of used ports back to file
	err = WriteUsedPorts(usedPortsFile, usedPorts)
	if err != nil {
		return 0, fmt.Errorf("failed to write updated used ports file: %w", err)
	}

	// Update the 'next_port' file to be the current assigned port + 1 for the next service
	err = ioutil.WriteFile(nextPortFile, []byte(strconv.Itoa(port+1)), 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to update next port file: %w", err)
	}

	return port, nil
}

// WriteUsedPortForService adds a user-provided port and service to the used ports file.
// This is used when a user explicitly specifies a port.
func WriteUsedPortForService(port int, serviceName, filename string) error {
	usedPorts, err := ReadUsedPorts(filename)
	if err != nil {
		return err
	}

	// Check if the port is already in the list to avoid duplicates
	if !IsPortUsed(usedPorts, port) {
		usedPorts.Ports = append(usedPorts.Ports, PortInfo{Port: port, Service: serviceName})
		return WriteUsedPorts(filename, usedPorts)
	}

	// If the port is already in the list, no action is needed (it's already tracked)
	return nil
}