package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools/formatting"
)

const (
	GoASTPath   = "path"
	GoASTSearch = "search"
)

type GoASTTool struct {
	ProjectPath string
}

func NewGoASTTool(projectPath, _ string) Tool {
	return &GoASTTool{ProjectPath: projectPath}
}

type fileInfo struct {
	path     string
	contents []byte
}

type parseResult struct {
	file     fileInfo
	parsed   *ast.File
	declInfo declInfo
}

type declInfo struct {
	declType    token.Token
	packageName string
	name        string
	startPos    token.Position
	endPos      token.Position
	// TODO: moar
}

func (g *GoASTTool) Name() string { return ToolGoAST }
func (g *GoASTTool) Description() string {
	examples := CollectExamples(g.Examples()...)

	return fmt.Sprintf(
		"Retrieve the definition of a type. Provide %q if you know what file/directory the definition is in, "+
			"though this parameter is optional. The parameter %q should contain the name of the type you want the "+
			"definition for. The tool will return the full definition with file path and line numbers.%s",
		GoASTPath, GoASTSearch, examples,
	)
}

func (g *GoASTTool) Examples() Examples {
	return Examples{
		{
			Description: `Get the type definition of the "Tool" type from the "pkg/tools" directory`,
			Args: Args{
				GoASTPath:   "pkg/tools",
				GoASTSearch: "Tool",
			},
		},
	}
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
			Key: GoASTSearch,
			Description: "The global type, function, or var/const definition to search for. Do not include 'type', " +
				"'func', etc, just provide the name of the identifier you want to find.",
			Type:     ParamTypeString,
			Required: true,
		},
	}
}

func (g *GoASTTool) Run(ctx context.Context, args Args) (string, error) {
	targetPath := g.ProjectPath

	// path is optional
	if path := args.GetString(GoFumptPath); path != nil {
		relPath, err := fs.GetRelativePath(g.ProjectPath, *path)
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

	parserCtx, cancel := context.WithCancel(ctx)
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
			if result.declInfo.name != *search {
				continue
			}

			matches = append(matches, result)
		}
	})

	resultWG.Go(func() {
		for err := range errChan {
			slog.Error("error received from parser worker", "error", err)
		}
	})

	walkCtx, walkCancel := context.WithTimeout(ctx, time.Second*60)
	defer walkCancel()

	iter, err := fs.ExcludesWalker(g.ProjectPath, targetPath)
	if err != nil {
		return "", fmt.Errorf("go_ast failed to walk files: %w", err)
	}

iterloop:
	for entry, err := range iter {
		select {
		case <-walkCtx.Done():
			break iterloop
		default:
		}

		if err != nil {
			slog.Error("error with entry while walking", "target_path", targetPath, "error", err)
			continue
		}

		if filepath.Ext(entry.Path) != ".go" {
			continue
		}

		contents, err := os.ReadFile(entry.Path)
		if err != nil {
			slog.Error("failed to read file", "path", entry.Path, "error", err)
			continue
		}

		fileChan <- fileInfo{
			path:     entry.Path,
			contents: contents,
		}
	}

	slog.Debug("done walking")

	close(fileChan)
	walkCancel()

	pw.wg.Wait()
	close(resultChan)
	close(errChan)

	resultWG.Wait()

	if len(matches) == 0 {
		return "No results", nil
	}

	buf := &bytes.Buffer{}
	for _, result := range matches {
		lines := bytes.Split(result.file.contents, []byte("\n"))
		matchedLines := lines[result.declInfo.startPos.Line-1 : result.declInfo.endPos.Line]
		content := formatting.WithLineNumbers(matchedLines, result.declInfo.startPos.Line)

		relPath, err := filepath.Rel(g.ProjectPath, result.file.path)
		if err == nil {
			fmt.Fprintf(buf, "%s (package = %s)\n%s\n", relPath, result.declInfo.packageName, LineSep)
		}

		buf.Write(content)
		buf.WriteByte('\n')
	}

	return buf.String(), nil
}

type parserWorker struct {
	wg   sync.WaitGroup
	fset *token.FileSet
}

func newParserWorker() *parserWorker {
	return &parserWorker{
		wg:   sync.WaitGroup{},
		fset: token.NewFileSet(),
	}
}

func (p *parserWorker) start(ctx context.Context, fileChan <-chan fileInfo, resultChan chan<- parseResult, errChan chan<- error) {
	for {
		select {
		case file, ok := <-fileChan:
			if !ok {
				return
			}

			parsed, err := parser.ParseFile(p.fset, file.path, file.contents, parser.SkipObjectResolution)
			if err != nil {
				errChan <- fmt.Errorf("failed to parse file %q: %w", file.path, err)
			}

			// TODO: use ast.FilterDecl here?
			for _, decl := range parsed.Decls {
				switch decl := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						result := parseResult{
							file:   file,
							parsed: parsed,
						}

						if spec == nil {
							continue
						}

						switch spec := spec.(type) {
						//  TODO: comments
						case *ast.ImportSpec:
							if spec.Name == nil {
								continue
							}

							result.declInfo = declInfo{
								declType:    decl.Tok,
								packageName: parsed.Name.Name,
								name:        spec.Name.Name,
								startPos:    p.fset.Position(spec.Pos()),
								endPos:      p.fset.Position(spec.End()),
							}
							resultChan <- result

						case *ast.TypeSpec:
							if spec.Name == nil {
								continue
							}

							result.declInfo = declInfo{
								declType:    decl.Tok,
								packageName: parsed.Name.Name,
								name:        spec.Name.Name,
								startPos:    p.fset.Position(spec.Pos()),
								endPos:      p.fset.Position(spec.End()),
							}
							resultChan <- result

						case *ast.ValueSpec:
							for _, name := range spec.Names {
								result.declInfo = declInfo{
									declType:    decl.Tok,
									packageName: parsed.Name.Name,
									name:        name.Name,
									startPos:    p.fset.Position(spec.Pos()),
									endPos:      p.fset.Position(spec.End()),
								}
								resultChan <- result
							}
						}
					}
				case *ast.FuncDecl:
					result := parseResult{
						file:   file,
						parsed: parsed,
						declInfo: declInfo{
							declType:    token.FUNC,
							packageName: parsed.Name.Name,
							name:        decl.Name.Name,
							startPos:    p.fset.Position(decl.Pos()),
							endPos:      p.fset.Position(decl.End()),
						},
					}

					resultChan <- result
				}
			}

		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		}
	}
}
