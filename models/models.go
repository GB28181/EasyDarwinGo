package models

import (
	"github.com/EasyDarwin/EasyDarwin/utils"
	_ "github.com/go-sql-driver/mysql" // mysql driver
	"github.com/jinzhu/gorm"
)

var DB *gorm.DB

// Init model modules
func Init() (err error) {
	DB, err = gorm.Open(config.Database.Type, config.Database.URL)
	if nil != err {
		return
	}

	DB.AutoMigrate(User{}, Stream{})
	count := 0
	DB.Model(User{}).Where("username = ?", config.HTTP.DefaultUsername).Count(&count)

	hashedPassword := utils.MD5(config.HTTP.DefaultPassword)

	if count == 0 {
		DB.Create(&User{
			Username: config.HTTP.DefaultUsername,
			Password: hashedPassword,
		})
	}
	return
}

// Close database
func Close() {
	if nil != DB {
		DB.Close()
	}
}
