// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"net/http"

	//
	"github.com/mark3labs/mcp-go/server"
)

type ToolMiddleware interface {
	Middleware(next server.ToolHandlerFunc) server.ToolHandlerFunc
}

type HttpMiddleware interface {
	Middleware(next http.Handler) http.Handler
}
