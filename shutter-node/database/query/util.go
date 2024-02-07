package query

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrAmbiguousResult = errors.New("multiple entries found")

func getObjByColumn[T any](db *gorm.DB, obj *T, name string, value any) (*T, error) {
	result := db.Where(fmt.Sprintf("%s = ?", name), value).Take(obj)
	return CheckGetUniqueObject(obj, result)
}

func CheckGetUniqueObject[T any](obj *T, result *gorm.DB) (*T, error) {
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	if result.RowsAffected > 1 {
		// should be unique
		return nil, ErrAmbiguousResult
	}
	return obj, nil
}
