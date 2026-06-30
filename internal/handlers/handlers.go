// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

package handlers

import "mcp-forge/internal/globals"

type HandlersManagerDependencies struct {
	AppCtx *globals.ApplicationContext
}

type HandlersManager struct {
	dependencies HandlersManagerDependencies
}

func NewHandlersManager(deps HandlersManagerDependencies) *HandlersManager {
	return &HandlersManager{
		dependencies: deps,
	}
}
