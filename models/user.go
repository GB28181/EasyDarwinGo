package models

type User struct {
	ID       int    `structs:"id" gorm:"primary_key" form:"id" json:"id"`
	Username string `gorm:"type:TEXT"`
	Password string `gorm:"type:TEXT"`
	Role     string `gorm:"type:TEXT"`
	Reserve1 string `gorm:"type:TEXT"`
	Reserve2 string `gorm:"type:TEXT"`
}
