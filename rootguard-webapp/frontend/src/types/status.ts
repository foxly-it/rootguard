// =====================================================
// File: frontend/src/types/status.ts
// Purpose: Central status type definitions for RootGuard
// =====================================================

/**
 * Backend health state as returned by API
 * This mirrors what the Go backend will expose.
 */
export type BackendHealthStatus = "ok" | "warn" | "error";

/**
 * UI-specific service state.
 * This is used by components like StatusIndicator.
 */
export type ServiceStatus = "healthy" | "degraded" | "down";