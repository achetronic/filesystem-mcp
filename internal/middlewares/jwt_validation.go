package middlewares

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	//
	"mcp-forge/internal/globals"

	//
	"github.com/google/cel-go/cel"
)

// JWTValidationMiddleware validates incoming JWTs against a JWKS endpoint.
//
// When enabled:
//   - Reads the token from the Authorization: Bearer header
//   - Validates signature using JWKS (fetched and cached from jwks_uri)
//   - Evaluates optional CEL allow_conditions against the JWT payload
//
// When disabled, all requests pass through without any token check.
type JWTValidationMiddleware struct {
	dependencies JWTValidationMiddlewareDependencies

	// Carried stuff
	jwks  *JWKS
	mutex sync.Mutex

	//
	celPrograms []*cel.Program
}

// JWTValidationMiddlewareDependencies holds the dependencies for the JWT validation middleware
type JWTValidationMiddlewareDependencies struct {
	AppCtx *globals.ApplicationContext
}

// NewJWTValidationMiddleware creates a new JWTValidationMiddleware.
// When jwt.enabled is true, it starts the JWKS cache worker and precompiles CEL expressions.
func NewJWTValidationMiddleware(deps JWTValidationMiddlewareDependencies) (*JWTValidationMiddleware, error) {

	mw := &JWTValidationMiddleware{
		dependencies: deps,
	}

	// Launch JWKS cache worker only when middleware is enabled
	if mw.dependencies.AppCtx.Config.Middleware.JWT.Enabled {
		go mw.cacheJWKS()
	}

	// Precompile CEL expressions to fail-fast and safe resources.
	allowConditionsEnv, err := cel.NewEnv(
		cel.Variable("payload", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("CEL environment creation error: %s", err.Error())
	}

	for _, allowCondition := range mw.dependencies.AppCtx.Config.Middleware.JWT.AllowConditions {

		ast, issues := allowConditionsEnv.Compile(allowCondition.Expression)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("CEL expression compilation exited with error: %s", issues.Err())
		}

		prg, err := allowConditionsEnv.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("CEL program construction error: %s", err.Error())
		}
		mw.celPrograms = append(mw.celPrograms, &prg)
	}

	return mw, nil
}

// Middleware returns an HTTP handler that validates the JWT on every request.
func (mw *JWTValidationMiddleware) Middleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		if !mw.dependencies.AppCtx.Config.Middleware.JWT.Enabled {
			goto nextStage
		}

		// Add WWW-Authenticate header just in case it is needed.
		// Will be cleared for authorized requests later.
		// Ref: https://modelcontextprotocol.io/specification/draft/basic/authorization
		{
			wwwAuthResourceMetadataUrl := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource%s",
				getRequestScheme(req), req.Host, mw.dependencies.AppCtx.Config.OAuthProtectedResource.UrlSuffix)
			wwwAuthScope := strings.Join(mw.dependencies.AppCtx.Config.OAuthProtectedResource.ScopesSupported, " ")

			rw.Header().Set("WWW-Authenticate",
				`Bearer error="invalid_token", `+
					`resource_metadata="`+wwwAuthResourceMetadataUrl+`", `+
					`scope="`+wwwAuthScope+`"`)
		}

		// 1. Extract token from Authorization header
		{
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(rw, "JWT: Access Denied: Authorization header not found", http.StatusUnauthorized)
				return
			}
			tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

			// 2. Validate token signature against JWKS
			_, err := mw.isTokenValid(tokenString)
			if err != nil {
				http.Error(rw, fmt.Sprintf("JWT: Access Denied: Invalid token: %v", err.Error()), http.StatusUnauthorized)
				return
			}

			// 3. Decode payload for CEL evaluation
			tokenStringParts := strings.Split(tokenString, ".")

			tokenPayloadBytes, err := base64.RawURLEncoding.DecodeString(tokenStringParts[1])
			if err != nil {
				mw.dependencies.AppCtx.Logger.Error("error decoding JWT payload from base64", "error", err.Error())
				http.Error(rw, "JWT: Access Denied: JWT Payload can not be decoded", http.StatusUnauthorized)
				return
			}

			tokenPayload := map[string]any{}
			err = json.Unmarshal(tokenPayloadBytes, &tokenPayload)
			if err != nil {
				mw.dependencies.AppCtx.Logger.Error("error decoding JWT payload from JSON", "error", err.Error())
				http.Error(rw, "JWT: Access Denied: Internal Issue", http.StatusUnauthorized)
				return
			}

			// 4. Evaluate CEL allow_conditions against the JWT payload
			for _, celProgram := range mw.celPrograms {
				out, _, err := (*celProgram).Eval(map[string]interface{}{
					"payload": tokenPayload,
				})

				if err != nil {
					mw.dependencies.AppCtx.Logger.Error("CEL program evaluation error", "error", err.Error())
					http.Error(rw, "JWT: Access Denied: Internal Issue", http.StatusUnauthorized)
					return
				}

				if out.Value() != true {
					http.Error(rw, "JWT: Access Denied: JWT does not meet conditions", http.StatusUnauthorized)
					return
				}
			}

			// Store validated token in request header for downstream use (e.g. RBAC)
			req.Header.Set("X-Validated-Jwt", tokenString)
		}

	nextStage:
		rw.Header().Del("WWW-Authenticate")
		next.ServeHTTP(rw, req)
	})
}
