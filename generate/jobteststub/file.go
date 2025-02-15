package jobteststub

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"log"
	"strings"

	infra_shared "github.com/magicpantry/infra/shared"
)

func Build(paths infra_shared.Paths, repo string) string {
	fs := token.NewFileSet()

	var imports []ast.Spec
	var statements []ast.Stmt

	imports = append(imports, &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: wrap("testing"),
		},
	})
	imports = append(imports, &ast.ImportSpec{
		Name: &ast.Ident{},
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: "",
		},
	})
	imports = append(imports, &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: wrap(fmt.Sprintf("github.com/magicpantry/%s/%s/mockmanifest", repo, infra_shared.MakeRelativeToRoot(paths.GenDir, paths))),
		},
	})

	statements = append(statements, &ast.AssignStmt{
		Tok: token.DEFINE,
		Lhs: []ast.Expr{&ast.Ident{Name: "tests"}},
		Rhs: []ast.Expr{&ast.CompositeLit{
			Type: &ast.ArrayType{
				Elt: &ast.StructType{
					Fields: &ast.FieldList{
						List: []*ast.Field{
							&ast.Field{
								Names: []*ast.Ident{&ast.Ident{Name: "Name"}},
								Type:  &ast.Ident{Name: "string"},
							},
							&ast.Field{
								Names: []*ast.Ident{&ast.Ident{Name: "Case"}},
								Type:  &ast.Ident{Name: "func(*testing.T, *mockmanifest.MockManifest)"},
							},
						},
					},
				},
			},
			Elts: []ast.Expr{
				&ast.CompositeLit{
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Name"},
							Value: &ast.Ident{Name: wrap("happy")},
						},
						&ast.KeyValueExpr{
							Key:   &ast.Ident{Name: "Case"},
							Value: &ast.Ident{Name: "testHandleHappy"},
						},
					},
				},
			},
		}},
	})
	statements = append(statements, &ast.ExprStmt{X: &ast.Ident{}})
	statements = append(statements, &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "test"},
		Tok:   token.DEFINE,
		X:     &ast.Ident{Name: "tests"},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Tok: token.DEFINE,
					Lhs: []ast.Expr{&ast.Ident{Name: "mm"}},
					Rhs: []ast.Expr{&ast.Ident{Name: "mockmanifest.NewMockManifest(t)"}},
				},
				&ast.ExprStmt{
					X: &ast.Ident{Name: "t.Run(test.Name, func(t *testing.T) { test.Case(t, mm) })"},
				},
			},
		},
	})

	f := &ast.File{
		Name: &ast.Ident{Name: "handlers_test"},
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok:   token.IMPORT,
				Specs: imports,
			},
			&ast.FuncDecl{
				Name: &ast.Ident{Name: "TestHandle"},
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						List: []*ast.Field{
							&ast.Field{
								Names: []*ast.Ident{&ast.Ident{Name: "t"}},
								Type:  &ast.Ident{Name: "*testing.T"},
							},
						},
					},
				},
				Body: &ast.BlockStmt{
					List: statements,
				},
			},
			&ast.FuncDecl{
				Name: &ast.Ident{Name: "testHandleHappy"},
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						List: []*ast.Field{
							&ast.Field{
								Names: []*ast.Ident{&ast.Ident{Name: "t"}},
								Type:  &ast.Ident{Name: "*testing.T"},
							},
							&ast.Field{
								Names: []*ast.Ident{&ast.Ident{Name: "mm"}},
								Type:  &ast.Ident{Name: "*mockmanifest.MockManifest"},
							},
						},
					},
				},
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.ExprStmt{X: &ast.Ident{Name: "// TODO"}},
					},
				},
			},
		},
	}

	buf := new(bytes.Buffer)
	if err := format.Node(buf, fs, f); err != nil {
		log.Fatal(err)
	}

	asString := string(buf.Bytes())

	return strings.ReplaceAll(asString, "\nfunc ", "\n\nfunc ")
}

func wrap(x string) string {
	return "\"" + x + "\""
}
