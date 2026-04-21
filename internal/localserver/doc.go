package localserver

// @title           cs-cloud API
// @version         1.0.0
// @description     cs-cloud local server REST API. Provides runtime file operations, terminal management, agent lifecycle, and proxies conversation/permission/question requests to the agent backend.
// @host            localhost:{port}
// @BasePath        /api/v1
// @securityDefinitions.apikey WorkspaceHeader
// @in header
// @name X-Workspace-Directory
// @description Workspace root directory for path sandbox resolution
