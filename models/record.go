package models

// Record of local
type Record struct {
	ID string `form:"id" json:"id" binding:"required"`
	// PlayPath in this server
	PlayPath string `form:"path" json:"path" binding:"required"`
}
