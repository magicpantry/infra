package manifestfile

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/shared"
	infra_shared "github.com/magicpantry/infra/shared"
)

type ManifestTemplate struct {
	Imports      []shared.Import
	Aliases      []shared.NameTypePair
	Configs      []shared.NameTypePair
	Dependencies []shared.NameTypePair
}

func configItemToNameTypePair(ci *proto.ConfigItem, configPlugins map[string]*proto.ConfigPluginOutput) *shared.NameTypePair {
	if ci.GetPluginValue() != nil {
		return &shared.NameTypePair{
			Name: ci.Name,
			Type: fmt.Sprintf("%s", configPlugins[ci.Name].ManifestType),
		}
	}
	if ci.GetIntValue() != 0 {
		return &shared.NameTypePair{
			Name: ci.Name,
			Type: "int",
		}
	}
	if ci.GetDoubleValue() != 0 {
		return &shared.NameTypePair{
			Name: ci.Name,
			Type: "float64",
		}
	}
	if ci.GetListValue() != nil {
		return &shared.NameTypePair{
			Name: ci.Name,
			Type: "[]string",
		}
	}
	if ci.GetStringValue() != "" {
		return &shared.NameTypePair{
			Name: ci.Name,
			Type: "string",
		}
	}
	return nil
}

func Build(paths infra_shared.Paths, mf *proto.Manifest, repo string, configPlugins map[string]*proto.ConfigPluginOutput) string {
	tmpl := shared.LoadTemplate(paths, "manifest.tmpl")
	manifestTemplate := ManifestTemplate{}

	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	rootConfigs := map[string]*proto.ConfigItem{}
	for _, item := range root.Config.Items {
		rootConfigs[item.Name] = item
	}

	if mf.Config != nil {
		for _, config := range mf.Config.Items {
			if config.GetKeyValue() != nil {
				item, ok := rootConfigs[config.Name]
				if !ok {
					local, ok := infra_shared.ReadConfigInOtherManifest(config.Name, paths.RootDir+"/"+config.GetKeyValue().Path)
					if !ok {
						log.Fatal("config item missing in root: " + config.Name)
					}
					item = local
				}
				ntp := configItemToNameTypePair(item, configPlugins)
				if ntp == nil {
					continue
				}
				manifestTemplate.Configs = append(manifestTemplate.Configs, *ntp)
			} else {
				ntp := configItemToNameTypePair(config, configPlugins)
				if ntp == nil {
					continue
				}
				manifestTemplate.Configs = append(manifestTemplate.Configs, *ntp)
			}
			if config.GetPluginValue() != nil {
				for _, importInfo := range configPlugins[config.Name].ManifestImports {
					manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
						Import:     importInfo,
						ImportType: shared.ImportTypeLocal,
					})
				}
			}
		}
	}
	for _, dep := range mf.RuntimeDependencies.Items {
		if dep.GetIp() != nil {
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "string",
				})
		}
		if dep.GetChan() != nil {
			for _, importInfo := range dep.GetChan().Type.Imports {
				manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
					ImportAs:   importInfo.ImportAs,
					Import:     importInfo.ImportName,
					ImportType: shared.ImportTypeLocal,
				})
			}
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: fmt.Sprintf("chan %s", dep.GetChan().Type.Name),
				})
		}
		if dep.GetGenerativeModel() != nil {
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/genai",
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "genai.GenAI",
				})
		}
		if dep.GetChrome() != nil {
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "context",
				ImportType: shared.ImportTypeGo,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "context.Context",
				})
		}
		if dep.GetGrpcClient() != nil {
			path, name := parseImportParts(dep.GetGrpcClient().Definition)
			importAs := strings.Join(popLast(strings.Split(strings.Split(path, ".")[0], "/")), "_")
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				ImportAs: importAs,
				Import: fmt.Sprintf(
					"github.com/magicpantry/%s/gen/%s",
					repo,
					parseImportPath(path)),
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: fmt.Sprintf("%s.%sClient", importAs, name),
				})
		}
		if dep.GetBlockstore() != nil {
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/blockstore",
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "blockstore.BlockStore",
				})
		}
		if dep.GetDatabase() != nil {
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/database",
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "database.Database",
				})
		}
		if dep.GetDocstore() != nil {
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/docstore",
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "docstore.DocStore",
				})
		}
		if dep.GetPool() != nil {
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/pool",
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: "pool.Pool",
				})
		}
		if dep.GetElastic() != nil {
			sig := dep.GetElastic().Signature
			keyPath, keyName := parseImportParts(sig.KeyType)
			filterPath, filterName := parseImportParts(sig.FilterType)
			sortPath, sortName := parseImportParts(sig.SortType)

			keyImportAs := strings.Join(popLast(strings.Split(strings.Split(keyPath, ".")[0], "/")), "_")
			filterImportAs := strings.Join(popLast(strings.Split(strings.Split(filterPath, ".")[0], "/")), "_")
			sortImportAs := strings.Join(popLast(strings.Split(strings.Split(sortPath, ".")[0], "/")), "_")

			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/textsearch",
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				ImportAs: keyImportAs,
				Import: fmt.Sprintf(
					"github.com/magicpantry/%s/gen/%s",
					repo,
					parseImportPath(keyPath)),
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				ImportAs: filterImportAs,
				Import: fmt.Sprintf(
					"github.com/magicpantry/%s/gen/%s",
					repo,
					parseImportPath(filterPath)),
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				ImportAs: sortImportAs,
				Import: fmt.Sprintf(
					"github.com/magicpantry/%s/gen/%s",
					repo,
					parseImportPath(sortPath)),
				ImportType: shared.ImportTypeLocal,
			})

			alias := fmt.Sprintf("%s_TextSearch", dep.Name)

			manifestTemplate.Aliases = append(
				manifestTemplate.Aliases, shared.NameTypePair{
					Name: alias,
					Type: fmt.Sprintf(
						"textsearch.TextSearch[*%s.%s, *%s.%s, *%s.%s]",
						keyImportAs, keyName,
						filterImportAs, filterName,
						sortImportAs, sortName),
				})

			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: alias,
				})
		}
		if dep.GetGrpcClientFactory() != nil {
			factory := dep.GetGrpcClientFactory()

			parts := strings.Split(factory.Definition, ":")

			withoutProto := strings.Split(parts[0], ".proto")[0]
			importPathParts := strings.Split(withoutProto, "/")
			withoutSuffix := importPathParts[:len(importPathParts)-1]

			importAs := strings.Join(withoutSuffix, "_")
			importPath := strings.Join(withoutSuffix, "/")

			alias := fmt.Sprintf("%s_GRPCClientFactory", dep.Name)

			manifestTemplate.Aliases = append(manifestTemplate.Aliases, shared.NameTypePair{
				Name: alias,
				Type: fmt.Sprintf(
					"func(context.Context, string) (%s.%sClient, error)",
					importAs, parts[1]),
			})

			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				Import:     "context",
				ImportType: shared.ImportTypeGo,
			})
			manifestTemplate.Imports = append(manifestTemplate.Imports, shared.Import{
				ImportAs:   importAs,
				Import:     fmt.Sprintf("github.com/magicpantry/%s/gen/%s", repo, importPath),
				ImportType: shared.ImportTypeLocal,
			})
			manifestTemplate.Dependencies = append(
				manifestTemplate.Dependencies, shared.NameTypePair{
					Name: dep.Name,
					Type: alias,
				})
		}
	}

	manifestTemplate.Imports = makeUniqueImports(manifestTemplate.Imports)

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, manifestTemplate); err != nil {
		log.Fatal(err)
	}

	return string(buf.Bytes())
}

func parseImportParts(x string) (string, string) {
	parts := strings.Split(x, ":")
	return parts[0], parts[1]
}

func popLast(xs []string) []string {
	return xs[:len(xs)-1]
}

func parseImportPath(x string) string {
	parts := strings.Split(x, "/")
	return strings.Join(parts[:len(parts)-1], "/")
}

func makeUniqueImports(xs []shared.Import) []shared.Import {
	set := map[shared.Import]any{}
	for _, x := range xs {
		set[x] = struct{}{}
	}
	var filtered []shared.Import
	for x := range set {
		filtered = append(filtered, x)
	}
	return filtered
}
