package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/camelcase"
	"github.com/fatih/structtag"
	"github.com/go-yaml/yaml"
)

type Doc struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Docs string `yaml:"docs"`
}

type Struct struct {
	SourcePath  string `yaml:"source_path"`
	Name        string `yaml:"name"`
	Required    []Doc  `yaml:"required"`
	NotRequired []Doc  `yaml:"not_required"`
}

const OuputDir = "website/data"

func main() {
	args := flag.Args()
	if len(args) == 0 {
		// Default: process the file
		args = []string{os.Getenv("GOFILE")}
	}
	fname := args[0]

	absFilePath, err := filepath.Abs(fname)
	if err != nil {
		panic(err)
	}
	paths := strings.SplitAfter(absFilePath, "packer"+string(os.PathSeparator))
	packerDir := paths[0]
	builderName, _ := filepath.Split(paths[1])
	builderName = strings.TrimSuffix(builderName, string(os.PathSeparator))
	builderName = strings.Join(strings.Split(builderName, string(os.PathSeparator)), "-")

	b, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Printf("ReadFile: %+v", err)
		os.Exit(1)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fname, b, parser.ParseComments)
	if err != nil {
		fmt.Printf("ParseFile: %+v", err)
		os.Exit(1)
	}

	for _, decl := range f.Decls {
		typeDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		typeSpec, ok := typeDecl.Specs[0].(*ast.TypeSpec)
		if !ok {
			continue
		}
		structDecl, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		fields := structDecl.Fields.List
		str := Struct{
			SourcePath: paths[1],
			Name:       typeSpec.Name.Name,
		}

		for _, field := range fields {
			if len(field.Names) == 0 || field.Tag == nil {
				continue
			}
			tag := field.Tag.Value[1:]
			tag = tag[:len(tag)-1]
			tags, err := structtag.Parse(tag)
			if err != nil {
				fmt.Printf("structtag.Parse(%s): err: %v", field.Tag.Value, err)
				os.Exit(1)
			}

			required := false
			if req, err := tags.Get("required"); err == nil && req.Value() == "true" {
				required = true
			}

			mstr, err := tags.Get("mapstructure")
			if err != nil {
				continue
			}
			name := mstr.Name

			var docs string
			if field.Doc != nil {
				docs = field.Doc.Text()
			} else {
				docs = strings.Join(camelcase.Split(field.Names[0].Name), " ")
			}

			doc := Doc{
				Name: name,
				Type: fmt.Sprintf("%s", b[field.Type.Pos()-1:field.Type.End()-1]),
				Docs: docs,
			}
			if required {
				str.Required = append(str.Required, doc)
			} else {
				str.NotRequired = append(str.NotRequired, doc)
			}
		}
		if len(str.Required) == 0 && len(str.NotRequired) == 0 {
			continue
		}

		outputPath := filepath.Join(packerDir, "website", "data", builderName+"-"+str.Name+".yml")

		outputFile, err := os.Create(outputPath)
		if err != nil {
			panic(err)
		}
		defer outputFile.Close()

		err = yaml.NewEncoder(outputFile).Encode(str)
		if err != nil {
			fmt.Printf("encode: %v", err)
			os.Exit(1)
		}
	}

}
