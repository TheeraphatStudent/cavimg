// Command cavimg-mcp is a stdio MCP server that helps an AI agent adopt the
// cavimg npm package into a frontend project.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"cavimg-mcp/internal/tools"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cavimg-mcp",
		Version: "0.1.0",
	}, nil)

	tools.Register(server)

	err := server.Run(context.Background(), &mcp.StdioTransport{})
	if isCleanShutdown(err) {
		return
	}
	log.Printf("cavimg-mcp: %v", err)
	os.Exit(1)
}

// isCleanShutdown reports whether a Server.Run error represents a normal
// stdio shutdown (the client closed the connection), rather than a real failure.
// A stdio server is launched per session, so client disconnect is expected.
func isCleanShutdown(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return true
	}
	// On stdin EOF the SDK's jsonrpc2 layer returns an (unexported, internal)
	// "server is closing" error that doesn't preserve the io.EOF chain. Match its
	// stable message since the value lives in a package we cannot import.
	msg := err.Error()
	return strings.Contains(msg, "server is closing") || strings.Contains(msg, "client is closing")
}
