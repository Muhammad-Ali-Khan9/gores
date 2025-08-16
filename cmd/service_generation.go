package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateData passed to templates for rendering.
type TemplateData struct {
	Name    string
	Port    string
	RootDir string
}

// createAuthMicroservice scaffolds the 'auth-service' specific folders and files
// from templates located exclusively in "templates/auth/".
func createAuthMicroservice(name = "Auth", port string) error {
	const templateRoot = "templates/auth/" // Hardcoded path for auth templates

	serviceDirPath := filepath.Join("services", name)
	internalDirPath := filepath.Join(serviceDirPath, "src/internal")
	cmdDirPath := filepath.Join(serviceDirPath, "src/cmd")

	// Create base service directory and internal/cmd subdirectories
	foldersToCreate := []string{
		serviceDirPath,
		internalDirPath,
		cmdDirPath,
	}

	for _, folder := range foldersToCreate {
		if err := os.MkdirAll(folder, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create folder %s: %w", folder, err)
		}
	}

	templates := map[string]string{
		"main.tmpl":       filepath.Join(cmdDirPath, "main.go"),
		"router.tmpl":     filepath.Join(internalDirPath, "router.go"),
		"controller.tmpl": filepath.Join(internalDirPath, "controller.go"),
		"service.tmpl":    filepath.Join(internalDirPath, "service.go"),
		"go.mod.tmpl":     filepath.Join(serviceDirPath, "go.mod"),
		"Dockerfile.tmpl": filepath.Join(serviceDirPath, "Dockerfile"),
		"entity.tmpl": filepath.Join("pkg", "entities", fmt.Sprintf("user.entity.go", name)),
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	rootDir := filepath.Base(cwd) // This assumes 'gores' is the current working directory base name.

	data := TemplateData{
		Name:    name,
		Port:    port,
		RootDir: rootDir,
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"title": strings.Title,
	}

	for tmplFile, outputPath := range templates {
		actualTemplatePath := templateRoot + tmplFile
		if tmplFile == "entity.tmpl" {
			actualTemplatePath = "templates/auth/" + tmplFile
		}

		content, err := templatesFS.ReadFile(actualTemplatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s from %s: %w", tmplFile, actualTemplatePath, err)
		}

		t, err := template.New(tmplFile).Funcs(funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", tmplFile, err)
		}

		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath, err)
		}

		err = t.Execute(f, data)
		f.Close() // Close the file immediately after execution
		if err != nil {
			return fmt.Errorf("failed to execute template %s into %s: %w", tmplFile, outputPath, err)
		}
		fmt.Printf("Generated: %s\n", outputPath)
	}

	goSumPath := filepath.Join(serviceDirPath, "go.sum")
	if _, err := os.Stat(goSumPath); os.IsNotExist(err) {
		if err := os.WriteFile(goSumPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create go.sum for service '%s': %w", name, err)
		}
		fmt.Printf("Generated: %s\n", goSumPath)
	}

	return nil
}

// createSharedPkg creates common shared folders (pkg/entities, pkg/database, pkg/http/middleware)
// and their corresponding files from embedded templates.
// It uses the global 'templatesFS' variable (defined in cmd/generate.go).
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
		// NOTE: Customize the module path here if your monorepo root module is not 'gores'
		pkgGoModContent := []byte("module gores/pkg\n\ngo 1.24\n")
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

	dbOutputPath := filepath.Join(dbPkgPath, "postgres", "connection.go") // Assume postgres subfolder
	if err := os.MkdirAll(filepath.Dir(dbOutputPath), os.ModePerm); err != nil { // Create parent dirs
		return fmt.Errorf("failed to create pkg/database/postgres folder: %w", err)
	}

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
		fmt.Println("pkg/database/postgres/connection.go created.")
	} else {
		fmt.Println("pkg/database/postgres/connection.go already exists, skipping creation.")
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

	middlewareOutputPath := filepath.Join(httpPkgPath, "middleware", "middleware.go") // Assume middleware subfolder
	if err := os.MkdirAll(filepath.Dir(middlewareOutputPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create pkg/http/middleware folder: %w", err)
	}

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
		fmt.Println("pkg/http/middleware/middleware.go created.")
	} else {
		fmt.Println("pkg/http/middleware/middleware.go already exists, skipping creation.")
	}

	return nil
}

// createMicroservice scaffolds microservice folders and files from embedded templates.
// It uses the global templatesFS variable and now accepts the specific template directory.
func createMicroservice(name, port, templateRoot string) error {
	serviceDirPath := filepath.Join("services", name)
	internalDirPath := filepath.Join(serviceDirPath, "internal")
	cmdDirPath := filepath.Join(serviceDirPath, "cmd")

	// Create base service directory and internal/cmd subdirectories
	foldersToCreate := []string{
		serviceDirPath,
		internalDirPath,
		cmdDirPath,
	}

	for _, folder := range foldersToCreate {
		if err := os.MkdirAll(folder, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create folder %s: %w", folder, err)
		}
	}

	// Map of template file names to their target output paths
	// The key is the template name relative to templateRoot
	templates := map[string]string{
		"main.tmpl":       filepath.Join(cmdDirPath, "main.go"),
		"router.tmpl":     filepath.Join(internalDirPath, "router.go"),
		"controller.tmpl": filepath.Join(internalDirPath, "controller.go"),
		"service.tmpl":    filepath.Join(internalDirPath, "service.go"),
		"go.mod.tmpl":     filepath.Join(serviceDirPath, "go.mod"),
		"Dockerfile.tmpl": filepath.Join(serviceDirPath, "Dockerfile"),
		// entities are shared, so they always come from the base 'templates/'
		"entity_pkg.tmpl": filepath.Join("pkg", "entities", fmt.Sprintf("%s.entity.go", name)),
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	rootDir := filepath.Base(cwd) // This assumes 'gores' is the current working directory base name.

	data := TemplateData{
		Name:    name,
		Port:    port,
		RootDir: rootDir,
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"title": strings.Title,
	}

	for tmplFile, outputPath := range templates {
		// For entity_pkg.tmpl, always use the base "templates/" path
		// as entities are generic and not service-specific template variations.
		actualTemplatePath := templateRoot + tmplFile
		if tmplFile == "entity_pkg.tmpl" {
			actualTemplatePath = "templates/" + tmplFile
		}

		content, err := templatesFS.ReadFile(actualTemplatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s from %s: %w", tmplFile, actualTemplatePath, err)
		}

		t, err := template.New(tmplFile).Funcs(funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", tmplFile, err)
		}

		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath, err)
		}

		err = t.Execute(f, data)
		f.Close() // Close the file immediately after execution
		if err != nil {
			return fmt.Errorf("failed to execute template %s into %s: %w", tmplFile, outputPath, err)
		}
		fmt.Printf("Generated: %s\n", outputPath)
	}

	// Create empty go.sum file for the new service
	goSumPath := filepath.Join(serviceDirPath, "go.sum")
	if _, err := os.Stat(goSumPath); os.IsNotExist(err) {
		if err := os.WriteFile(goSumPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create go.sum for service '%s': %w", name, err)
		}
		fmt.Printf("Generated: %s\n", goSumPath)
	}

	return nil
}