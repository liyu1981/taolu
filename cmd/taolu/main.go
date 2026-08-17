// Command taolu runs the versioned practice library as an MCP server.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yli/taolu/pkg/tools"
	"github.com/yli/taolu/pkg/vault"
	"github.com/yli/taolu/pkg/web"
)

const (
	serverName    = "taolu"
	serverVersion = "0.1.0"
	defaultHost   = "127.0.0.1"
	defaultPort   = 8264
)

func main() {
	stdio := flag.Bool("stdio", false, "run as a stdio MCP server instead of HTTP")
	mcpOnly := flag.Bool("mcp-only", false, "run only the MCP HTTP server (disable the web UI)")
	webOnly := flag.Bool("web-only", false, "run only the web UI server (disable the MCP server)")
	flag.Parse()

	if *mcpOnly && *webOnly {
		log.Fatal("--mcp-only and --web-only are mutually exclusive")
	}
	if *stdio && *webOnly {
		log.Fatal("--stdio and --web-only are mutually exclusive")
	}

	if err := initVaultAtStartup(); err != nil {
		log.Fatalf("initialize vault: %v", err)
	}

	var shutdowns []func()
	switch {
	case *stdio:
		runStdio()
		return
	case *webOnly:
		shutdowns = append(shutdowns, startWebServer())
	case *mcpOnly:
		shutdowns = append(shutdowns, startMCPServer())
	default:
		shutdowns = append(shutdowns, startMCPServer())
		if stop := startWebServer(); stop != nil {
			shutdowns = append(shutdowns, stop)
		}
	}
	waitForShutdown(shutdowns)
}

// initVaultAtStartup creates and seeds the default vault if it does not exist,
// so the server is usable immediately without a separate taolu_init call.
func initVaultAtStartup() error {
	r, p, err := vault.EnsureVault("", "")
	if err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	log.Printf("vault ready at %s", p)
	return nil
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
		Version: serverVersion,
	}, nil)

	tools.RegisterTaoluTools(server)

	return server, nil
}
