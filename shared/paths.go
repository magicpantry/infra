package shared

import (
	"log"
	"os"
	"strings"
)

type Paths struct {
	InfraDir     string
	RootDir      string
	ComponentDir string
	GenDir       string
	WorkspaceDir string
	MainPath     string

	PrefixFromGenToRoot string
}

func MakeRelativeToRoot(path string, paths Paths) string {
	return strings.TrimPrefix(path, paths.RootDir+"/")
}

func MakePaths(componentDir string) Paths {
	rootDir := findRootDir()

	serviceRelativeDir := strings.TrimPrefix(componentDir, rootDir+"/")

	workspaceDir := rootDir + "/.workspace/" + serviceRelativeDir
	genDir := rootDir + "/gen/" + serviceRelativeDir

	infraDir := rootDir + "/infra"
	if _, err := os.Stat(infraDir + "/generate"); err != nil {
		parts := strings.Split(rootDir, "/")
		infraDir = strings.Join(parts[:len(parts)-1], "/") + "/infra"
	}

	return Paths{
		InfraDir:            infraDir,
		RootDir:             rootDir,
		ComponentDir:        componentDir,
		GenDir:              genDir,
		WorkspaceDir:        workspaceDir,
		PrefixFromGenToRoot: buildPrefix(serviceRelativeDir),
	}
}

func buildPrefix(serviceDir string) string {
	// Add 1 for the ".workspace root.
	prefixCount := len(strings.Split(serviceDir, "/")) + 1
	var prefixParts []string
	for i := 0; i < prefixCount; i++ {
		prefixParts = append(prefixParts, "..")
	}
	return strings.Join(prefixParts, "/")
}

func currentDir() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return wd
}

func RootDir() string {
	return findRootDir()
}

func findRootDir() string {
	wd := currentDir()
	parts := strings.Split(wd, "/")[1:]

	for i := len(parts); i >= 0; i-- {
		path := "/" + strings.Join(parts[0:i], "/")
		if _, err := os.Stat(path + "/root.textproto"); err == nil {
			return path
		}
	}

	log.Fatal("couldn't find project root")
	return ""
}
