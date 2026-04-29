package localserver

import "net/http"

// favoriteItem represents a single cloud favorite item with its current status.
type favoriteItem struct {
	ID          string `json:"id" example:"abc123"`
	Slug        string `json:"slug" example:"my-skill"`
	Name        string `json:"name" example:"My Skill"`
	Description string `json:"description" example:"A useful skill"`
	ItemType    string `json:"itemType" example:"skill"`
	Status      string `json:"status" example:"Active"`
	LocalPath   string `json:"localPath,omitempty" example:"/home/user/.config/costrict/skills/my-skill"`
}

// favoriteActionResponse represents the response from a favorite load/unload action.
type favoriteActionResponse struct {
	Success bool   `json:"success" example:"true"`
	Slug    string `json:"slug" example:"my-skill"`
}

// @Summary      List favorite items
// @Description  List all cloud favorite items with their current status. Supports filtering by type via query parameter.
// @Tags         Agent
// @Produce      json
// @Param        type   query   string  false  "Filter by item type: skill, agent, command, mcp"
// @Success      200    {array}  favoriteItem
// @Failure      500    {object} envelope
// @Router       /agents/favorites [get]
func (s *Server) handleFavoriteList(w http.ResponseWriter, r *http.Request) {
	s.handleProxy(w, r)
}

// @Summary      Load a favorite item
// @Description  Load a favorite skill, agent, command, or MCP into the current workspace.
// @Tags         Agent
// @Produce      json
// @Param        id     path    string  true  "Favorite item slug or ID"
// @Success      200    {object}  favoriteActionResponse
// @Failure      400    {object} envelope
// @Failure      500    {object} envelope
// @Router       /agents/favorites/{id}/load [post]
func (s *Server) handleFavoriteLoad(w http.ResponseWriter, r *http.Request) {
	s.handleProxy(w, r)
}

// @Summary      Unload a favorite item
// @Description  Unload a favorite skill, agent, command, or MCP from the current workspace.
// @Tags         Agent
// @Produce      json
// @Param        id     path    string  true  "Favorite item slug or ID"
// @Success      200    {object}  favoriteActionResponse
// @Failure      400    {object} envelope
// @Failure      500    {object} envelope
// @Router       /agents/favorites/{id}/unload [post]
func (s *Server) handleFavoriteUnload(w http.ResponseWriter, r *http.Request) {
	s.handleProxy(w, r)
}
