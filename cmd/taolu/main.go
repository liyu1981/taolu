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
	stdio := flag.Bool("stdio", false, "run as a stdio MCP server (default is HTTP)")
	flag.Parse()

	if err := initVaultAtStartup(); err != nil {
		log.Fatalf("initialize vault: %v", err)
	}

	if *stdio {
		runStdio()
		return
	}
	runHTTP()
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

func runHTTP() {
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

	stopWeb := runWebServer()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown: %v; closing", err)
		if cerr := httpServer.Close(); cerr != nil {
			log.Printf("close: %v", cerr)
		}
	}
	if stopWeb != nil {
		stopWeb()
	}
}

// runWebServer starts the browser UI on the MCP port + 1. It is disabled when
// TAOLU_WEB_PORT is "0". It returns a stop function, or nil if not started.
func runWebServer() func() {
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
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := webServer.Shutdown(ctx); err != nil {
			_ = webServer.Close()
		}
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
