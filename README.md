# Go Microservice Boilerplate CLI

[![Go](https://img.shields.io/badge/go-1.20-blue.svg)](https://golang.org)
[![GitHub Release](https://img.shields.io/github/v/release/Muhammad-Ali-Khan9/gores)](https://github.com/your-username/go-microservice-boilerplate/releases)
[![License](https://img.shields.io/github/license/Muhammad-Ali-Khan9/gores)](LICENSE)

---

## Introduction

**Go Microservice framework** is a command-line tool designed to rapidly generate microservice boilerplate code for Go backends. It scaffolds a production-ready project structure including routers, controllers, services, entities, Dockerfiles, and essential configuration files. The generated services follow best practices for modular and maintainable microservice development.

This tool supports generating RESTful microservices with plans to extend support for GraphQL and gRPC APIs.

---

## License

This project is licensed under the [MIT License](LICENSE).  
You are free to use, modify, and distribute this software — including for commercial purposes — under the terms of the MIT License.


---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to report issues, request features, and submit pull requests.


---

## Features

- Generate boilerplate code for Go microservices with a clean architecture  
- Auto-assign or specify network ports intelligently avoiding conflicts  
- Create shared packages for entities, database connections, and HTTP middleware  
- CLI-based interaction powered by [Cobra](https://github.com/spf13/cobra)  
- Cross-platform support with pre-built binaries for Linux, macOS, and Windows  
- Automated GitHub Actions workflow for seamless releases

---

## Installation

### 1. Install via Go CLI (source install)

If you have Go installed, you can directly install the CLI tool using:

```bash
go install github.com/your-username/go-microservice-boilerplate/cmd@latest
```

This will compile and install the gores executable in your $GOPATH/bin or $HOME/go/bin directory.
Make sure your Go bin path is in your PATH environment variable:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

You can then run:

```bash
gores generate myservice
```

### 2. Download pre-built binaries

You can download pre-compiled binaries from the [GitHub Releases page](https://github.com/your-username/go-microservice-boilerplate/releases).

Available binaries:

- `gores-linux-amd64` (Linux)  
- `gores-darwin-amd64` (macOS)  
- `gores-windows-amd64.exe` (Windows)  

**Usage example (Linux/macOS):**

```bash
chmod +x gores-linux-amd64
./gores-linux-amd64 generate myservice
```

**Usage example (Windows PowerShell):**

```Powershell
.\gores-windows-amd64.exe generate myservice
```

## Usage

```bash
gores generate [service-name] [port]
```

 - service-name: The name of the microservice to generate (required).
 - port: (Optional) port number. If omitted, the CLI automatically assigns the next available port starting from 8080.

 On running the command, you will be prompted to select the API type (currently only RESTful is supported):
 ```bash
 Select API type to generate:
1) Restful
2) GraphQL
3) gRPC
Enter choice (1-3):
```