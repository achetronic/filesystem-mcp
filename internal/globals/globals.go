package globals

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"mcp-forge/api"
	"mcp-forge/internal/config"
)

type ApplicationContext struct {
	Context    context.Context
	Logger     *slog.Logger
	Config     *api.Configuration
	ToolPrefix string
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func SanitizeToolPrefix(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return ""
	}
	return s + "_"
}

const defaultServerName = "filesystem-mcp"

func NewApplicationContext() (*ApplicationContext, error) {

	appCtx := &ApplicationContext{
		Context: context.Background(),
		Logger:  slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}

	// Parse and store the config
	var configFlag = flag.String("config", "config.yaml", "path to the config file")
	flag.Parse()

	configContent, err := config.ReadFile(*configFlag)
	if err != nil {
		return appCtx, err
	}
	appCtx.Config = &configContent
	serverName := configContent.Server.Name
	if serverName == "" {
		serverName = defaultServerName
	}
	appCtx.ToolPrefix = SanitizeToolPrefix(serverName)

	//
	return appCtx, nil
}
