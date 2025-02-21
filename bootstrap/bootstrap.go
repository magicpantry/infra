package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/magicpantry/infra/shared"
)

func filterBlock(content string) []string {
	lines := strings.Split(content, "\n")
	var filtered []string
	foundStart := false
	foundEnd := false
	for _, line := range lines {
		didFindEnd := false
		if line == "### infra START ###" {
			foundStart = true
		}
		if foundStart && line == "### infra END ###" {
			didFindEnd = true
		}
		if foundStart && !foundEnd {
			continue
		}
		filtered = append(filtered, line)
		foundEnd = didFindEnd
	}
	return filtered
}

func main() {
	paths := shared.MakePaths("")

	shared.Run(fmt.Sprintf("cd %s/bootstrap && mkdir -p bin && go build -o bin/run", paths.InfraDir))
	shared.Run(fmt.Sprintf("cd %s/build && mkdir -p bin && go build -o bin/run", paths.InfraDir))
	shared.Run(fmt.Sprintf("cd %s/generate && mkdir -p bin && go build -o bin/run", paths.InfraDir))
	shared.Run(fmt.Sprintf("cd %s/run && mkdir -p bin && go build -o bin/run", paths.InfraDir))
	shared.Run(fmt.Sprintf("cd %s/test && mkdir -p bin && go build -o bin/run", paths.InfraDir))
	shared.Run(fmt.Sprintf("cd %s/endpoints && mkdir -p bin && go build -o bin/run", paths.InfraDir))

	bs, err := os.ReadFile(os.ExpandEnv("$HOME/.bashrc"))
	if err != nil {
		log.Fatal(err)
	}

	filtered := filterBlock(string(bs))
	filtered = append(filtered, "### infra START ###")

	filtered = append(filtered, fmt.Sprintf("alias infra_bootstrap='%s/bootstrap/bin/run'", paths.InfraDir))
	filtered = append(filtered, fmt.Sprintf("alias infra_build='%s/build/bin/run'", paths.InfraDir))
	filtered = append(filtered, fmt.Sprintf("alias infra_generate='%s/generate/bin/run'", paths.InfraDir))
	filtered = append(filtered, fmt.Sprintf("alias infra_run='%s/run/bin/run'", paths.InfraDir))
	filtered = append(filtered, fmt.Sprintf("alias infra_test='%s/test/bin/run'", paths.InfraDir))
	filtered = append(filtered, fmt.Sprintf("alias infra_endpoints='%s/endpoints/bin/run'", paths.InfraDir))

	filtered = append(filtered, "### infra END ###")

	combined := strings.Join(filtered, "\n")

	if err := os.WriteFile(os.ExpandEnv("$HOME/.bashrc"), []byte(combined), 0644); err != nil {
		log.Fatal(err)
	}

	log.Println("don't forget to `source ~/.bashrc`")
}
