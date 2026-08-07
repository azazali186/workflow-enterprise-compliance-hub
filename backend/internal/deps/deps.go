// Package deps defines the shared dependency container handed to every module
// at route registration time.
package deps

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	sagacore "github.com/aeroxe/compliance-hub/backend/internal/saga"
	"github.com/aeroxe/compliance-hub/backend/internal/ws"
)

// Deps bundles the services every module needs.
type Deps struct {
	DB     *gorm.DB
	Bus    bus.Bus
	Cache  cache.Cache
	Hub    *ws.Hub
	Logger *slog.Logger
	// Saga is the README Saga Orchestrator engine (ComplianceCheck,
	// AuditExecution, ViolationProcessing, CorrectiveActionFlow).
	Saga *sagacore.Engine
}
