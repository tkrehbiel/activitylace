package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLite_Add(t *testing.T) {
	json1 := []byte(`{"id":1,"published":"2022-11-22T07:43:00Z","content":"hello"}`)
	json2 := []byte(`{"id":1,"published":"2022-11-22T07:43:00Z","content":"goodbye"}`)
	obj := NewMapObject(json1)
	d := sqliteCollection{name: "test", connection: "file::memory:?cache=shared"}
	openErr := d.Open()
	require.NoError(t, openErr)
	defer d.Close()

	assert.NoError(t, d.Upsert(context.Background(), obj))

	objects, err := d.SelectAll(context.Background())
	assert.NoError(t, err)
	require.Equal(t, 1, len(objects))
	assert.Equal(t, obj.ID(), objects[0].ID())

	obj2 := NewMapObject(json2)
	assert.NoError(t, d.Upsert(context.Background(), obj2))

	objects, err = d.SelectAll(context.Background())
	assert.NoError(t, err)
	require.Equal(t, 1, len(objects))
	assert.Equal(t, obj.ID(), objects[0].ID())
}
