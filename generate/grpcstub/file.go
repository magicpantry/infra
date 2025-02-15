package grpcstub

import (
	"fmt"
	"sort"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/shared"
	infra_shared "github.com/magicpantry/infra/shared"
)

func Build(paths infra_shared.Paths, grpcServerManifest *proto.GrpcServer, rpc shared.RPCInfo, repo string) string {
	imports := shared.BuildImports(paths, grpcServerManifest, []shared.RPCInfo{rpc}, repo)
	var goImports, localImports, externalImports []string
	for _, imp := range imports {
		if imp.ImportType == shared.ImportTypeGo {
			goImports = append(goImports, importToString(imp))
		}
		if imp.ImportType == shared.ImportTypeLocal {
			localImports = append(localImports, importToString(imp))
		}
		if imp.ImportType == shared.ImportTypeExternal {
			externalImports = append(externalImports, importToString(imp))
		}
	}

	serviceName := strings.Split(grpcServerManifest.Definition, ":")[1]

	signature := rpc.Name + "("
	if rpc.IsStreamInput {
		signature += "server proto." + serviceName + "_" + rpc.Name + "Server, mf *manifest.Manifest"
	} else {
		inputType := "proto." + rpc.InputType

		if strings.Contains(rpc.InputType, ".") {
			parts := strings.Split(rpc.InputType, ".")
			importAs := strings.ToLower(strings.Join(parts, "_"))
			inputType = importAs + "." + parts[len(parts)-1]
		}

		signature += "ctx context.Context, arg *" + inputType + ", mf *manifest.Manifest"
	}
	signature += ") "
	var returnCount int
	if rpc.IsStreamInput {
		signature += "error"
		returnCount = 1
	} else {
		outputType := "proto." + rpc.OutputType

		if strings.Contains(rpc.OutputType, ".") {
			parts := strings.Split(rpc.OutputType, ".")
			importAs := strings.ToLower(strings.Join(parts, "_"))
			outputType = importAs + "." + parts[len(parts)-1]
		}

		signature += fmt.Sprintf("(*%s, error)", outputType)
		returnCount = 2
	}

	body := "package handlers\n\n"
	body += buildImportBlock(goImports, externalImports, localImports) + "\n\n"
	body += fmt.Sprintf("func %s {\n", signature)
	if returnCount == 1 {
		body += "\treturn nil\n"
	} else {
		body += "\treturn nil, nil\n"
	}
	body += "}\n"

	return body
}

func makeUnique(xs []string) []string {
	set := map[string]any{}
	for _, x := range xs {
		set[x] = struct{}{}
	}
	var filtered []string
	for x := range set {
		filtered = append(filtered, x)
	}
	sort.Strings(filtered)
	return filtered
}

func buildImportBlock(goImports, externalImports, localImports []string) string {
	goImports = makeUnique(goImports)
	externalImports = makeUnique(externalImports)
	localImports = makeUnique(localImports)

	var imports []string
	if len(goImports) > 0 {
		for i := range goImports {
			goImports[i] = "\t" + goImports[i]
		}
		imports = append(imports, strings.Join(goImports, "\n"))
	}
	if len(externalImports) > 0 {
		for i := range externalImports {
			externalImports[i] = "\t" + externalImports[i]
		}
		imports = append(imports, strings.Join(externalImports, "\n"))
	}
	if len(localImports) > 0 {
		for i := range localImports {
			localImports[i] = "\t" + localImports[i]
		}
		imports = append(imports, strings.Join(localImports, "\n"))
	}
	return "import (\n" + strings.Join(imports, "\n\n") + "\n)"
}

func importToString(imp shared.Import) string {
	if imp.ImportAs != "" {
		return fmt.Sprintf("%s \"%s\"", imp.ImportAs, imp.Import)
	}
	return fmt.Sprintf("\"%s\"", imp.Import)
}
