package main

import (
	"github.com/MikkoParkkola/trvl/mcp"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	var (
		httpMode bool
		host     string
		port     int
		token    string
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP (Model Context Protocol) server",
		Long: `Start the trvl MCP server for AI agent integration.

By default, runs in stdio mode (JSON-RPC over stdin/stdout) for use with
Claude Code and other MCP-compatible clients.

Use --http to start an HTTP server instead. It listens on 127.0.0.1 by
default; pass --host explicitly for gateway or remote access. HTTP mode
requires bearer-token authentication, using --token, TRVL_MCP_TOKEN, or a
random token generated at startup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if httpMode {
				return mcp.RunHTTP(host, port, token)
			}
			return mcp.Run()
		},
	}

	cmd.Flags().BoolVar(&httpMode, "http", false, "Run as HTTP server instead of stdio")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "HTTP server host (only used with --http)")
	cmd.Flags().IntVar(&port, "port", 8080, "HTTP server port (only used with --http)")
	cmd.Flags().StringVar(&token, "token", "", "Bearer token for HTTP mode (default: TRVL_MCP_TOKEN or generated)")

	cmd.AddCommand(mcpInstallCmd())

	return cmd
}
