package middlewares

import "context"

// contextKey is an unexported type for context keys in this package.
type contextKey string

// jwtPayloadKey is the context key under which the verified JWT payload is stored.
const jwtPayloadKey contextKey = "jwt_payload"

// ContextWithJWTPayload returns a new context carrying the verified JWT payload.
func ContextWithJWTPayload(ctx context.Context, payload map[string]any) context.Context {
	return context.WithValue(ctx, jwtPayloadKey, payload)
}

// JWTPayloadFromContext retrieves the verified JWT payload from the context.
// Returns nil if JWT validation was disabled or the payload is not present.
func JWTPayloadFromContext(ctx context.Context) map[string]any {
	payload, _ := ctx.Value(jwtPayloadKey).(map[string]any)
	return payload
}
