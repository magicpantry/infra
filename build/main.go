package main

import (
	"fmt"
	"log"
	"os"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/shared"
)

func main() {
	for componentDir, manifest := range shared.ReadManifest() {
		log.Printf("build '%s/%s'\n", manifest.Component.Namespace, manifest.Component.Name)
		runMain(componentDir, manifest)
	}
}

func runMain(componentDir string, manifest *proto.Manifest) {
	if !isBuildable(manifest) {
		return
	}

	paths := shared.MakePaths(componentDir)
	if err := os.MkdirAll(paths.WorkspaceDir+"/bin", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	shared.Run(fmt.Sprintf("cd %s/cmd && go build -o %s/bin/%s", paths.GenDir, paths.WorkspaceDir, manifest.Component.Name))
}

func isBuildable(mf *proto.Manifest) bool {
	buildables := []bool{
		mf.Component.GetGrpcServer() != nil,
		mf.Component.GetHttpServer() != nil,
		mf.Component.GetJob() != nil,
	}
	is := false
	for _, b := range buildables {
		is = is || b
	}
	return is
}
