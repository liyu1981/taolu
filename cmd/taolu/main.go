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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yli/taolu/pkg/tools"
	"github.com/yli/taolu/pkg/vault"
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	if err := httpServer.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// listenAddr resolves the bind address from TAOLU_HOST and TAOLU_PORT.
// Defaults to 127.0.0.1:8264.
func listenAddr() string {
	host := os.Getenv("TAOLU_HOST")
	if host == "" {
		host = defaultHost
	}
	port := defaultPort
	if p := os.Getenv("TAOLU_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 65535 {
			port = n
		} else {
			log.Printf("invalid TAOLU_PORT %q, using default %d", p, defaultPort)
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func newServer() (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	tools.RegisterTaoluTools(server)

	return server, nil
}
