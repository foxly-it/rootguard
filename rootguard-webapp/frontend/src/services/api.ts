// =====================================================
// RootGuard API Service
// =====================================================

export interface VersionResponse {
  version: string;
  commit: string;
}

export async function getVersion(): Promise<VersionResponse> {
  const response = await fetch("/api/version");

  if (!response.ok) {
    throw new Error("Failed to fetch version");
  }

  return response.json();
}