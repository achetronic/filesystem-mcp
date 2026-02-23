package main

import (
	"log"
	"net/http"
	"time"

	//
	"mcp-forge/internal/globals"
	"mcp-forge/internal/handlers"
	"mcp-forge/internal/middlewares"
	"mcp-forge/internal/rbac"
	"mcp-forge/internal/state"
	"mcp-forge/internal/tools"

	//
	"github.com/mark3labs/mcp-go/server"
)

func main() {

	// 0. Process the configuration
	appCtx, err := globals.NewApplicationContext()
	if err != nil {
		log.Fatalf("failed creating application context: %v", err.Error())
	}

	// 1. Initialize middlewares that need it
	accessLogsMw := middlewares.NewAccessLogsMiddleware(middlewares.AccessLogsMiddlewareDependencies{
		AppCtx: appCtx,
	})

	jwtValidationMw, err := middlewares.NewJWTValidationMiddleware(middlewares.JWTValidationMiddlewareDependencies{
		AppCtx: appCtx,
	})
	if err != nil {
		appCtx.Logger.Info("failed starting JWT validation middleware", "error", err.Error())
	}

	// 2. Initialize RBAC engine
	rbacEngine, err := rbac.NewEngine(appCtx)
	if err != nil {
		log.Fatalf("failed creating RBAC engine: %v", err.Error())
	}

	// 3. Initialize shared state
	undoStore := state.NewUndoStore()
	scratchStore := state.NewScratchStore()
	processStore := state.NewProcessStore()

	// 4. Create a new MCP server
	mcpServer := server.NewMCPServer(
		appCtx.Config.Server.Name,
		appCtx.Config.Server.Version,
		server.WithToolCapabilities(true),
	)

	// 5. Initialize handlers for later usage
	hm := handlers.NewHandlersManager(handlers.HandlersManagerDependencies{
		AppCtx: appCtx,
	})

	// 6. Add filesystem and system tools to the MCP server
	tm := tools.NewToolsManager(tools.ToolsManagerDependencies{
		AppCtx: appCtx,

		McpServer:   mcpServer,
		Middlewares: []middlewares.ToolMiddleware{},
		RBAC:        rbacEngine,
		Undo:        undoStore,
		Scratch:     scratchStore,
		Processes:   processStore,
	})
	tm.AddTools()

	// 7. Wrap MCP server in a transport (stdio, HTTP, SSE)
	switch appCtx.Config.Server.Transport.Type {
	case "http":
		httpServer := server.NewStreamableHTTPServer(mcpServer,
			server.WithHeartbeatInterval(30*time.Second),
			server.WithStateLess(false))

		mux := http.NewServeMux()
		mux.Handle("/mcp", accessLogsMw.Middleware(jwtValidationMw.Middleware(httpServer)))

		if appCtx.Config.OAuthAuthorizationServer.Enabled {
			mux.Handle("/.well-known/oauth-authorization-server"+appCtx.Config.OAuthAuthorizationServer.UrlSuffix,
				accessLogsMw.Middleware(http.HandlerFunc(hm.HandleOauthAuthorizationServer)))
		}

		if appCtx.Config.OAuthProtectedResource.Enabled {
			mux.Handle("/.well-known/oauth-protected-resource"+appCtx.Config.OAuthProtectedResource.UrlSuffix,
				accessLogsMw.Middleware(http.HandlerFunc(hm.HandleOauthProtectedResources)))
		}

		appCtx.Logger.Info("starting StreamableHTTP server", "host", appCtx.Config.Server.Transport.HTTP.Host)
		err := http.ListenAndServe(appCtx.Config.Server.Transport.HTTP.Host, mux)
		if err != nil {
			log.Fatal(err)
		}

	default:
		appCtx.Logger.Info("starting stdio server")
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatal(err)
		}
	}
}
