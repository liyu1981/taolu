package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yli/taolu/pkg/tools"
	"github.com/yli/taolu/pkg/version"
	"github.com/yli/taolu/pkg/web"
)

const (
	serverName = "taolu"
	defaultHost = "127.0.0.1"
	defaultPort = 8264
)

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	with := fs.String("with", "httpmcp,web", "comma-separated components to start: httpmcp, web, stdio")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: taolu serve [options]

Start the taolu MCP server and/or web UI.

Options:
  --with <components>  comma-separated list of components to start (default: httpmcp,web)
                       valid: httpmcp, web, stdio
  -h, --help           show this help message

Examples:
  taolu serve                       Start HTTP MCP + web UI
  taolu serve --with=httpmcp        Start HTTP MCP only
  taolu serve --with=web            Start web UI only
  taolu serve --with=stdio          Start in stdio mode (for agent integration)`)
	}
	fs.Parse(args)

	components := parseComponents(*with)

	if components["stdio"] {
		runStdio()
		return
	}

	var shutdowns []func()
	if components["httpmcp"] {
		shutdowns = append(shutdowns, startMCPServer())
	}
	if components["web"] {
		if stop := startWebServer(); stop != nil {
			shutdowns = append(shutdowns, stop)
		}
	}
	if len(shutdowns) == 0 {
		log.Fatal("no components to start; use --with=httpmcp,web or --with=stdio")
	}
	waitForShutdown(shutdowns)
}

// parseComponents splits a comma-separated --with value into a set.
func parseComponents(s string) map[string]bool {
	valid := map[string]bool{"httpmcp": true, "web": true, "stdio": true}
	result := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !valid[part] {
			log.Fatalf("unknown component %q (valid: httpmcp, web, stdio)", part)
		}
		result[part] = true
	}
	// stdio is exclusive
	if result["stdio"] && (result["httpmcp"] || result["web"]) {
		log.Fatal("--with=stdio is mutually exclusive with httpmcp and web")
	}
	return result
}

func runStdio() {
	server, err := newServer()
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}

// startMCPServer starts the MCP Streamable HTTP server and returns its
// shutdown function.
func startMCPServer() func() {
	server, err := newServer()
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{})
	httpServer := &http.Server{
		Addr:    listenAddr(),
		Handler: handler,
	}
	log.Printf("%s MCP server listening on http://%s", serverName, httpServer.Addr)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	return func() { shutdownServer(httpServer) }
}

// startWebServer starts the browser UI on the MCP port + 1. It is disabled when
// TAOLU_WEB_PORT is "0". It returns the web server's shutdown function, or nil
// when disabled.
func startWebServer() func() {
	host := os.Getenv("TAOLU_HOST")
	if host == "" {
		host = defaultHost
	}
	port := webPort()
	if port == 0 {
		log.Printf("web UI disabled (TAOLU_WEB_PORT=0)")
		return nil
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	webServer := &http.Server{
		Addr:    addr,
		Handler: web.NewHandler(""),
	}
	go func() {
		if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("web server failed: %v", err)
		}
	}()
	log.Printf("web UI listening on http://%s", webServer.Addr)
	return func() { shutdownServer(webServer) }
}

// shutdownServer gracefully stops an http.Server with a short timeout.
func shutdownServer(s *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown: %v; closing", err)
		if cerr := s.Close(); cerr != nil {
			log.Printf("close: %v", cerr)
		}
	}
}

// waitForShutdown blocks until SIGINT/SIGTERM, then runs the shutdown funcs.
func waitForShutdown(stops []func()) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	for _, s := range stops {
		s()
	}
}

// webPort returns the web UI port: TAOLU_WEB_PORT if set (0 disables), else the
// MCP port + 1.
func webPort() int {
	if p := os.Getenv("TAOLU_WEB_PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 65535 {
			log.Printf("invalid TAOLU_WEB_PORT %q, using default", p)
		} else {
			return n
		}
	}
	return portFromEnv() + 1
}

// portFromEnv resolves the MCP port from TAOLU_PORT (or the default).
func portFromEnv() int {
	port := defaultPort
	if p := os.Getenv("TAOLU_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 65535 {
			port = n
		}
	}
	return port
}

// listenAddr resolves the bind address from TAOLU_HOST and TAOLU_PORT.
// Defaults to 127.0.0.1:8264.
func listenAddr() string {
	host := os.Getenv("TAOLU_HOST")
	if host == "" {
		host = defaultHost
	}
	return net.JoinHostPort(host, strconv.Itoa(portFromEnv()))
}

func newServer() (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: version.Version,
	}, nil)

	tools.RegisterTaoluTools(server)

	return server, nil
}
