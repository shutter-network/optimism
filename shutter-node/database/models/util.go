package models

import "gorm.io/gorm"

type DBScope func(*gorm.DB) *gorm.DB
