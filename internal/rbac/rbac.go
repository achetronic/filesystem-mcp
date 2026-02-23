package rbac

import (
	"fmt"
	"path/filepath"
	"strings"

	//
	"mcp-forge/api"
	"mcp-forge/internal/globals"

	//
	"github.com/google/cel-go/cel"
)

type compiledRule struct {
	config   api.RBACRuleConfig
	programs []*cel.Program
}

type Engine struct {
	appCtx *globals.ApplicationContext
	rules  []compiledRule
}

func NewEngine(appCtx *globals.ApplicationContext) (*Engine, error) {
	engine := &Engine{
		appCtx: appCtx,
	}

	if !appCtx.Config.RBAC.Enabled {
		return engine, nil
	}

	env, err := cel.NewEnv(
		cel.Variable("payload", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating CEL environment for RBAC: %s", err.Error())
	}

	for _, rule := range appCtx.Config.RBAC.Rules {
		cr := compiledRule{
			config: rule,
		}

		for _, expr := range rule.When {
			ast, issues := env.Compile(expr)
			if issues != nil && issues.Err() != nil {
				return nil, fmt.Errorf("RBAC rule %q: CEL compilation error: %s", rule.Name, issues.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				return nil, fmt.Errorf("RBAC rule %q: CEL program error: %s", rule.Name, err.Error())
			}

			cr.programs = append(cr.programs, &prg)
		}

		engine.rules = append(engine.rules, cr)
	}

	return engine, nil
}

// operationCategory maps tool names to their operation category
var operationCategory = map[string]string{
	"ls":             "read",
	"read_file":      "read",
	"search":         "read",
	"diff":           "read",
	"write_file":     "write",
	"edit_file":      "write",
	"undo":           "write",
	"exec":           "exec",
	"process_status": "exec",
	"process_kill":   "exec",
}

// pathFreeTools are tools that don't operate on filesystem paths
var pathFreeTools = map[string]bool{
	"system_info": true,
	"scratch":     true,
}

// Check evaluates RBAC rules for a given tool invocation.
// jwtPayload can be nil when JWT is not enabled.
func (e *Engine) Check(toolName string, paths []string, jwtPayload map[string]any) error {
	if !e.appCtx.Config.RBAC.Enabled {
		return nil
	}

	if pathFreeTools[toolName] {
		return nil
	}

	category, ok := operationCategory[toolName]
	if !ok {
		return fmt.Errorf("access denied: unknown tool %q", toolName)
	}

	for _, path := range paths {
		if err := e.checkPath(category, path, jwtPayload); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) checkPath(operation string, path string, jwtPayload map[string]any) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("access denied: invalid path %q", path)
	}

	for _, rule := range e.rules {
		if !e.matchesWhen(rule, jwtPayload) {
			continue
		}

		if !matchesPath(rule.config.Paths, absPath) {
			continue
		}

		if matchesOperation(rule.config.Operations, operation) {
			return nil
		}
	}

	if e.appCtx.Config.RBAC.DefaultPolicy == "allow" {
		return nil
	}

	return fmt.Errorf("access denied: %s not allowed on %q", operation, path)
}

func (e *Engine) matchesWhen(rule compiledRule, jwtPayload map[string]any) bool {
	if len(rule.programs) == 0 {
		if jwtPayload == nil {
			return true
		}
		return true
	}

	if jwtPayload == nil {
		return false
	}

	for _, prg := range rule.programs {
		out, _, err := (*prg).Eval(map[string]interface{}{
			"payload": jwtPayload,
		})
		if err != nil {
			e.appCtx.Logger.Error("RBAC CEL evaluation error", "rule", rule.config.Name, "error", err.Error())
			return false
		}
		if out.Value() != true {
			return false
		}
	}

	return true
}

func matchesPath(patterns []string, absPath string) bool {
	for _, pattern := range patterns {
		if matchGlob(pattern, absPath) {
			return true
		}
	}
	return false
}

func matchGlob(pattern, path string) bool {
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	matched, _ := filepath.Match(pattern, path)
	return matched
}

func matchesOperation(operations []string, operation string) bool {
	for _, op := range operations {
		if op == operation {
			return true
		}
	}
	return false
}
