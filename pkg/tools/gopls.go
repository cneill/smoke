package tools

//
// import (
// 	"bufio"
// 	"bytes"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"log/slog"
// 	"net/url"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strconv"
// 	"strings"
// 	"time"
//
// 	"github.com/cneill/smoke/pkg/utils"
// )
//
// const (
// 	GoPLSPath               = "path"
// 	GoPLSLine               = "line"
// 	GoPLSColumn             = "column"
// 	GoPLSIncludeDeclaration = "include_declaration"
// )
//
// // GoPLSTool provides a minimal interface to gopls via LSP/JSON-RPC, supporting references.
// // It starts a gopls instance over stdio, opens the requested file, and issues a references request.
// //
// // Return value is the raw JSON array of LSP Location objects from textDocument/references.
// // If no results are found, it returns "[]".
// //
// // Limitations: this is a very small client focused only on what's needed for references.
// // It does not implement full LSP negotiation and ignores most server notifications.
// // Timeouts are conservative to avoid hanging indefinitely.
// //
// // Note: line and column are 1-based in the tool API and converted to 0-based for LSP positions.
// type GoPLSTool struct {
// 	ProjectPath string
// }
//
// func (g *GoPLSTool) Name() string { return ToolGoPLS }
//
// func (g *GoPLSTool) Description() string {
// 	return fmt.Sprintf(
// 		"Provides a minimal interface to gopls using LSP/JSON-RPC. Currently supports finding references for a "+
// 			"symbol. Required params: %q (file), %q (1-based), %q (1-based). Optional: %q (boolean). Returns JSON "+
// 			"array of LSP Locations.",
// 		GoPLSPath, GoPLSLine, GoPLSColumn, GoPLSIncludeDeclaration,
// 	)
// }
//
// func (g *GoPLSTool) Params() Params {
// 	return Params{
// 		{Key: GoPLSPath, Description: "Path to the Go source file", Type: ParamTypeString, Required: true},
// 		{Key: GoPLSLine, Description: "1-based line number", Type: ParamTypeNumber, Required: true},
// 		{Key: GoPLSColumn, Description: "1-based column number", Type: ParamTypeNumber, Required: true},
// 		{Key: GoPLSIncludeDeclaration, Description: "Include the declaration in the results", Type: ParamTypeBoolean, Required: false},
// 	}
// }
//
// func (g *GoPLSTool) Run(args Args) (string, error) { //nolint:funlen,cyclop
// 	if _, err := exec.LookPath("gopls"); err != nil {
// 		slog.Error("gopls not found on the system", "error", err)
// 		return "", fmt.Errorf("%w: gopls not found on the system", ErrFileSystem)
// 	}
//
// 	path := args.GetString(GoPLSPath)
// 	line := args.GetInt64(GoPLSLine)
// 	column := args.GetInt64(GoPLSColumn)
//
// 	if path == nil || line == nil || column == nil {
// 		return "", fmt.Errorf("%w: missing required parameter(s)", ErrArguments)
// 	}
//
// 	relPath, err := utils.GetRelativePath(g.ProjectPath, *path)
// 	if err != nil {
// 		return "", fmt.Errorf("%w: path error: %w", ErrArguments, err)
// 	}
//
// 	// read file contents to send in didOpen to ensure gopls has up-to-date view
// 	content, err := os.ReadFile(relPath)
// 	if err != nil {
// 		return "", fmt.Errorf("%w: failed to read file: %w", ErrFileSystem, err)
// 	}
//
// 	rootURI := fileURI(g.ProjectPath)
// 	pathURI := fileURI(relPath)
//
// 	cmd := exec.Command("gopls")
// 	cmd.Dir = g.ProjectPath
//
// 	stdin, err := cmd.StdinPipe()
// 	if err != nil {
// 		return "", fmt.Errorf("%w: failed to get stdin: %w", ErrFileSystem, err)
// 	}
//
// 	stdout, err := cmd.StdoutPipe()
// 	if err != nil {
// 		return "", fmt.Errorf("%w: failed to get stdout: %w", ErrFileSystem, err)
// 	}
//
// 	stderr := &bytes.Buffer{}
// 	cmd.Stderr = stderr
//
// 	if err := cmd.Start(); err != nil {
// 		return "", fmt.Errorf("%w: failed to start gopls: %w", ErrCommandExecution, err)
// 	}
//
// 	reader := bufio.NewReader(stdout)
// 	writer := bufio.NewWriter(stdin)
//
// 	// timeouts to prevent hanging forever
// 	// deadline := time.Now().Add(20 * time.Second)
// 	// _ = stdout.SetReadDeadline(deadline) // ignore error if not supported
//
// 	// message ids
// 	var (
// 		initID int64 = 1
// 		refID  int64 = 2
// 		shutID int64 = 3
// 	)
//
// 	procID := os.Getpid()
// 	initReq := map[string]any{
// 		"jsonrpc": "2.0",
// 		"id":      initID,
// 		"method":  "initialize",
// 		"params": map[string]any{
// 			"processId":    procID,
// 			"rootUri":      rootURI,
// 			"capabilities": map[string]any{},
// 		},
// 	}
//
// 	if err := writeLSPMessage(writer, initReq); err != nil {
// 		_ = cmd.Process.Kill()
// 		return "", fmt.Errorf("%w: failed to write initialize: %w", ErrCommandExecution, err)
// 	}
//
// 	// wait for initialize response
// 	if _, err := readResponseByID(reader, initID); err != nil {
// 		_ = cmd.Process.Kill()
// 		return "", fmt.Errorf("%w: initialize failed: %w (stderr: %s)", ErrCommandExecution, err, stderr.String())
// 	}
//
// 	// send initialized notification
// 	_ = writeLSPMessage(writer, map[string]any{
// 		"jsonrpc": "2.0",
// 		"method":  "initialized",
// 		"params":  map[string]any{},
// 	})
//
// 	// open the file with full content
// 	_ = writeLSPMessage(writer, map[string]any{
// 		"jsonrpc": "2.0",
// 		"method":  "textDocument/didOpen",
// 		"params": map[string]any{
// 			"textDocument": map[string]any{
// 				"uri":        pathURI,
// 				"languageId": "go",
// 				"version":    1,
// 				"text":       string(content),
// 			},
// 		},
// 	})
//
// 	includeDecl := false
// 	if b := args.GetBool(GoPLSIncludeDeclaration); b != nil {
// 		includeDecl = *b
// 	}
//
// 	// send references request
// 	req := map[string]any{
// 		"jsonrpc": "2.0",
// 		"id":      refID,
// 		"method":  "textDocument/references",
// 		"params": map[string]any{
// 			"textDocument": map[string]any{"uri": pathURI},
// 			"position":     map[string]any{"line": *line - 1, "character": *column - 1},
// 			"context":      map[string]any{"includeDeclaration": includeDecl},
// 		},
// 	}
// 	if err := writeLSPMessage(writer, req); err != nil {
// 		_ = cmd.Process.Kill()
// 		return "", fmt.Errorf("%w: failed to write references request: %w", ErrCommandExecution, err)
// 	}
//
// 	resp, err := readResponseByID(reader, refID)
// 	if err != nil {
// 		_ = cmd.Process.Kill()
// 		return "", fmt.Errorf("%w: references failed: %w (stderr: %s)", ErrCommandExecution, err, stderr.String())
// 	}
//
// 	// shutdown and exit politely
// 	_ = writeLSPMessage(writer, map[string]any{
// 		"jsonrpc": "2.0",
// 		"id":      shutID,
// 		"method":  "shutdown",
// 		"params":  map[string]any{},
// 	})
// 	_, _ = readResponseByID(reader, shutID)
// 	_ = writeLSPMessage(writer, map[string]any{
// 		"jsonrpc": "2.0",
// 		"method":  "exit",
// 	})
//
// 	// attempt to close cleanly
// 	_ = writer.Flush()
// 	_ = stdin.Close()
// 	_ = stdout.Close()
// 	_ = cmd.Process.Release()
//
// 	// extract result
// 	locationsRaw, ok := resp["result"]
// 	if !ok || locationsRaw == nil {
// 		return "[]", nil
// 	}
//
// 	// result can be []Location or null
// 	// if _, isNull := locationsRaw.(nil); isNull {
// 	// if locationsRaw == nil {
// 	// 	return "[]", nil
// 	// }
//
// 	out, err := json.Marshal(locationsRaw)
// 	if err != nil {
// 		// fall back to string representation
// 		return fmt.Sprintf("%v", locationsRaw), nil
// 	}
//
// 	return string(out), nil
// }
//
// // writeLSPMessage marshals v to JSON and writes a Content-Length framed LSP message.
// func writeLSPMessage(writer *bufio.Writer, v any) error {
// 	payload, err := json.Marshal(v)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal message to JSON: %w", err)
// 	}
//
// 	header := "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n"
// 	if _, err := writer.WriteString(header); err != nil {
// 		return fmt.Errorf("failed to write header: %w", err)
// 	}
//
// 	if _, err := writer.Write(payload); err != nil {
// 		return fmt.Errorf("failed to write message: %w", err)
// 	}
//
// 	if err := writer.Flush(); err != nil {
// 		return fmt.Errorf("failed to flush buffered writer to write message: %w", err)
// 	}
//
// 	return nil
// }
//
// // readResponseByID reads LSP messages until it finds a JSON-RPC response with the given id.
// // It returns the decoded message as a generic map.
// func readResponseByID(r *bufio.Reader, responseID int64) (map[string]any, error) {
// 	deadline := time.Now().Add(20 * time.Second)
// 	for time.Now().Before(deadline) {
// 		body, err := readLSPMessage(r)
// 		if err != nil {
// 			if errors.Is(err, io.EOF) {
// 				break
// 			}
//
// 			return nil, err
// 		}
//
// 		var msg map[string]any
// 		if err := json.Unmarshal(body, &msg); err != nil {
// 			continue
// 		}
//
// 		// notifications have no id
// 		rawID, hasID := msg["id"]
// 		if !hasID {
// 			continue
// 		}
// 		// id can be number or string per JSON-RPC
// 		switch val := rawID.(type) {
// 		case float64:
// 			if int64(val) == responseID {
// 				return msg, nil
// 			}
// 		case string:
// 			if val == strconv.FormatInt(responseID, 10) {
// 				return msg, nil
// 			}
// 		}
// 	}
//
// 	return nil, fmt.Errorf("timeout waiting for response id=%d", responseID)
// }
//
// // readLSPMessage reads a single LSP message body based on Content-Length header.
// func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
// 	// read headers
// 	contentLength := -1
//
// 	for {
// 		line, err := reader.ReadString('\n')
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to read headers: %w", err)
// 		}
//
// 		line = strings.TrimRight(line, "\r\n")
// 		if line == "" { // end of headers
// 			break
// 		}
//
// 		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
// 			parts := strings.SplitN(line, ":", 2)
// 			if len(parts) == 2 {
// 				v := strings.TrimSpace(parts[1])
// 				n, _ := strconv.Atoi(v)
// 				contentLength = n
// 			}
// 		}
// 	}
//
// 	if contentLength < 0 {
// 		return nil, fmt.Errorf("missing Content-Length header")
// 	}
//
// 	buf := make([]byte, contentLength)
// 	if _, err := io.ReadFull(reader, buf); err != nil {
// 		return nil, fmt.Errorf("failed to read message: %w", err)
// 	}
//
// 	return buf, nil
// }
//
// func fileURI(path string) string {
// 	if !filepath.IsAbs(path) {
// 		abs, _ := filepath.Abs(path)
// 		path = abs
// 	}
//
// 	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
//
// 	return u.String()
// }
