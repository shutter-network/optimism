package serializers

import (
	"context"
	"reflect"

	serializers "github.com/ethereum-optimism/optimism/indexer/database/serializers"
	"gorm.io/gorm/schema"
)

type BytesSerializer struct {
	serializers.BytesSerializer
}
type (
	BytesInterface    interface{ Bytes() []byte }
	SetBytesInterface interface{ SetBytes([]byte) }
)

func init() {
	schema.RegisterSerializer("bytes", BytesSerializer{
		BytesSerializer: serializers.BytesSerializer{},
	})
}

func (s BytesSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) error {
	return s.BytesSerializer.Scan(ctx, field, dst, dbValue)
}

func (s BytesSerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue interface{}) (interface{}, error) {
	return s.BytesSerializer.Value(ctx, field, dst, fieldValue)
}
