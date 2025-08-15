package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cneill/smoke/pkg/utils"
)

const (
	GoASTPath   = "path"
	GoASTSearch = "search"
)

type GoASTTool struct {
	ProjectPath string
}

type fileInfo struct {
	path     string
	contents []byte
}

type parseResult struct {
	path     string
	file     fileInfo
	parsed   *ast.File
	typeInfo typeInfo
}

type typeInfo struct {
	Package  string
	Name     string
	StartPos token.Position
	EndPos   token.Position
	// TODO: moar
}

func (g *GoASTTool) Name() string { return ToolGoAST }

func (g *GoASTTool) Description() string {
	return fmt.Sprintf(
		"Performs AST analysis of Go code to perform various actions." +
			GoFumptPath,
	)
}

func (g *GoASTTool) Params() Params {
	return Params{
		{
			Key:         GoASTPath,
			Description: "The path of the directory/file to search",
			Type:        ParamTypeString,
			Required:    false,
		},
		{
			Key:         GoASTSearch,
			Description: "The type definition to search for",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

// TODO: pass in a context to Run()
func (g *GoASTTool) Run(args Args) (string, error) {
	targetPath := g.ProjectPath

	// path is optional
	if path := args.GetString(GoFumptPath); path != nil {
		relPath, err := utils.GetRelativePath(g.ProjectPath, *path)
		if err != nil {
			return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
		}

		targetPath = relPath
	}

	search := args.GetString(GoASTSearch)
	if search == nil || *search == "" {
		return "", fmt.Errorf("%w: missing search", ErrArguments)
	}

	if _, err := os.Stat(targetPath); err != nil {
		return "", fmt.Errorf("%w: failed to stat path %q: %w", ErrFileSystem, targetPath, err)
	}

	fileChan := make(chan fileInfo)
	resultChan := make(chan parseResult)
	errChan := make(chan error)

	parserCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pw := newParserWorker()

	for range 5 {
		pw.wg.Go(func() {
			pw.start(parserCtx, fileChan, resultChan, errChan)
		})
	}

	resultWG := sync.WaitGroup{}

	matches := []parseResult{}

	resultWG.Go(func() {
		for result := range resultChan {
			if result.typeInfo.Name != *search {
				continue
			}

			slog.Debug("got result",
				"path", result.path,
				"package", result.typeInfo.Package,
				"name", result.typeInfo.Name,
				"start", result.typeInfo.StartPos,
				"end", result.typeInfo.EndPos,
			)

			matches = append(matches, result)
		}
	})

	ctx, walkCancel := context.WithTimeout(context.Background(), time.Second*60)

	walkErr := filepath.WalkDir(targetPath, g.walker(ctx, fileChan))
	if walkErr != nil {
		walkCancel()
		return "", fmt.Errorf("walk error: %w", walkErr)
	}

	slog.Debug("done walking")

	close(fileChan)
	walkCancel()

	pw.wg.Wait()
	close(resultChan)

	resultWG.Wait()

	buf := &bytes.Buffer{}
	for _, result := range matches {
		lines := bytes.Split(result.file.contents, []byte("\n"))
		matchedLines := lines[result.typeInfo.StartPos.Line-1 : result.typeInfo.EndPos.Line]
		content := utils.WithLineNumbers(matchedLines, result.typeInfo.StartPos.Line)

		relPath, err := filepath.Rel(g.ProjectPath, result.file.path)
		if err == nil {
			fmt.Fprintf(buf, "%s\n%s\n", relPath, LineSep)
		}

		buf.Write(content)
		buf.WriteByte('\n')
	}

	return buf.String(), nil
}

type parserWorker struct {
	wg         sync.WaitGroup
	fset       *token.FileSet
	typeConfig types.Config
}

func newParserWorker() *parserWorker {
	return &parserWorker{
		wg:         sync.WaitGroup{},
		fset:       token.NewFileSet(),
		typeConfig: types.Config{Importer: importer.Default()},
	}
}

func (p *parserWorker) start(ctx context.Context, fileChan <-chan fileInfo, resultChan chan<- parseResult, errChan chan<- error) {
	for {
		select {
		case file, ok := <-fileChan:
			if !ok {
				return
			}

			// slog.Debug("parsing file", "path", file.path)

			parsed, err := parser.ParseFile(p.fset, file.path, file.contents, parser.SkipObjectResolution)
			if err != nil {
				errChan <- fmt.Errorf("failed to parse file %q: %w", file.path, err)
			}

			for node := range ast.Preorder(parsed) {
				decl, ok := node.(ast.Decl)
				if !ok {
					continue
				}

				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}

				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						result := parseResult{
							path:   file.path,
							file:   file,
							parsed: parsed,
							typeInfo: typeInfo{
								Package:  parsed.Name.Name,
								Name:     typeSpec.Name.Name,
								StartPos: p.fset.Position(typeSpec.Pos()),
								EndPos:   p.fset.Position(typeSpec.End()),
							},
						}

						resultChan <- result
					}
				}
			}

		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		}
	}
}

func (g *GoASTTool) walker(ctx context.Context, fileChan chan<- fileInfo) fs.WalkDirFunc {
	return func(path string, dirEntry fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		if filepath.Ext(dirEntry.Name()) != ".go" {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%w: failed to read file %q: %w", ErrFileSystem, path, err)
		}

		f := fileInfo{
			path:     path,
			contents: contents,
		}

		fileChan <- f

		return nil
	}
}
