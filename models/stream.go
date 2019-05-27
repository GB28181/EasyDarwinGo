package models

// Stream of source
type Stream struct {
	ID                int    `gorm:"primary_key"`
	URL               string `gorm:"type:varchar(254);unique"`
	CustomPath        string `gorm:"type:varchar(254);unique"`
	IdleTimeout       int
	HeartbeatInterval int
}
