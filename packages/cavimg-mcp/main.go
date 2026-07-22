// Command cavimg-mcp is a stdio MCP server that helps an AI agent adopt the
// cavimg npm package into a frontend project.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"cavimg-mcp/internal/tools"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cavimg-mcp",
		Version: "0.1.0",
	}, nil)

	tools.Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("cavimg-mcp: %v", err)
	}
}
