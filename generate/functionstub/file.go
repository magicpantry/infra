package functionstub

import (
	"fmt"
	"sort"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/shared"
	infra_shared "github.com/magicpantry/infra/shared"
)

func Build(paths infra_shared.Paths, functionManifest *proto.Function, repo string) string {
	var goImports, localImports, externalImports []string
	goImports = append(localImports, "\"context\"")
	externalImports = append(externalImports, "\"github.com/cloudevents/sdk-go/v2/event\"")
	externalImports = append(externalImports, "\"github.com/googleapis/google-cloudevents-go/cloud/firestoredata\"")
	externalImports = append(externalImports, "\"google.golang.org/protobuf/proto\"")
	localImports = append(
		localImports,
		fmt.Sprintf("\"github.com/magicpantry/%s/%s/manifest\"", repo, infra_shared.MakeRelativeToRoot(paths.GenDir, paths)))

	signature := "Handle(ctx context.Context, event event.Event, mf *manifest.Manifest) error"

	body := "package handler\n\n"
	body += buildImportBlock(goImports, externalImports, localImports) + "\n\n"
	body += fmt.Sprintf("func %s {\n", signature)
	body += "\tvar data firestoredata.DocumentEventData\n"
	body += "\tif err := proto.Unmarshal(event.Data(), &data); err != nil {\n"
	body += "\t\treturn err\n"
	body += "\t}\n"
	body += "\n"
	body += "\tTODO\n"
	body += "\n"
	body += "\treturn nil\n"
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
