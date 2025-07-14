package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	err := filepath.Walk("internal", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		fmt.Printf("Processing file: %s\n", path)

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Printf("Error parsing file %s: %v\n", path, err)
			return nil
		}

		file.Comments = nil

		var buf bytes.Buffer
		err = format.Node(&buf, fset, file)
		if err != nil {
			fmt.Printf("Error formatting file %s: %v\n", path, err)
			return nil
		}

		err = ioutil.WriteFile(path, buf.Bytes(), info.Mode())
		if err != nil {
			fmt.Printf("Error writing file %s: %v\n", path, err)
			return nil
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Comment removal completed successfully!")
}