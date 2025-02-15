package makefile

import (
	"bytes"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/shared"
	infra_shared "github.com/magicpantry/infra/shared"
)

type MakefileTemplate struct {
	Prefix             string
	Targets            []string
	DirectoryTargets   []string
	GoProtoTargets     []string
	PythonProtoTargets []string
	PythonInitTargets  []string
}

func Build(component *proto.Component, paths infra_shared.Paths, prefix string, protos []string) string {
	tmpl := shared.LoadTemplate(paths, "makefile.tmpl")

	dirs := map[string][]string{}
	for _, proto := range protos {
		dirs[filepath.Dir(proto)] = append(dirs[filepath.Dir(proto)], proto)
	}

	var dirDependencies []string
	var protoDependencies []string
	var initDependencies []string
	var initTargets []string
	for dir, inDirs := range dirs {
		var processedInDirs []string
		for _, inDir := range inDirs {
			if component.GetModelServer() == nil {
				processedInDirs = append(processedInDirs, "gen/"+strings.TrimSuffix(inDir, ".proto")+".pb.go")
			} else {
				initDir := dir + "/proto"
				parts := strings.Split(initDir, "/")
				for i := range parts {
					base := strings.Join(parts[:i], "/")
					if base != "" {
						base = "/" + base
					}
					initDependencies = append(initDependencies, base+"/__init__.py")
					initTargets = append(initTargets, base)
				}
				processedInDirs = append(processedInDirs, "gen/"+strings.TrimSuffix(inDir, ".proto")+"_pb2.py")
				dirDependencies = append(dirDependencies, filepath.Dir(inDir))
			}
		}

		if component.GetModelServer() == nil {
			dirDependencies = append(dirDependencies, dir)
		}
		protoDependencies = append(protoDependencies, processedInDirs...)
	}
	sort.Strings(dirDependencies)
	sort.Strings(protoDependencies)
	sort.Strings(initDependencies)

	var prefixedInitDependencies []string
	for _, dep := range initDependencies {
		prefixedInitDependencies = append(prefixedInitDependencies, "gen"+dep)
	}

	var prefixedDirDependencies []string
	for _, dep := range dirDependencies {
		prefixedDirDependencies = append(prefixedDirDependencies, "gen/"+dep)
	}

	var goProtoTargets []string
	var pythonProtoTargets []string
	if component.GetModelServer() != nil {
		pythonProtoTargets = dirDependencies
	} else {
		goProtoTargets = dirDependencies
	}

	makefileTemplate := MakefileTemplate{
		Prefix:             prefix,
		Targets:            append(append(prefixedDirDependencies, protoDependencies...), prefixedInitDependencies...),
		DirectoryTargets:   dirDependencies,
		GoProtoTargets:     goProtoTargets,
		PythonProtoTargets: pythonProtoTargets,
		PythonInitTargets:  initTargets,
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, makefileTemplate); err != nil {
		log.Fatal(err)
	}

	return string(buf.Bytes())
}
