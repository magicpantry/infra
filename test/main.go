package main

import (
	"fmt"
	"log"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/shared"
)

func main() {
	for componentDir, manifest := range shared.ReadManifest() {
		log.Printf("test '%s/%s'\n", manifest.Component.Namespace, manifest.Component.Name)
		runMain(componentDir, manifest)
	}
}

func runMain(componentDir string, manifest *proto.Manifest) {
	if manifest.Component.GetGrpcServer() == nil && manifest.Component.GetFunction() == nil {
		return
	}

	paths := shared.MakePaths(componentDir)
	shared.Run(fmt.Sprintf("cd %s && go test ./...", paths.ComponentDir))
}
