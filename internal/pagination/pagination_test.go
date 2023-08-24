package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetaNormalizesLimitAndOffset(t *testing.T) {
	pg := New(0, -9, 2)

	assert.Equal(t, DefaultLimit, pg.Limit)
	assert.Equal(t, 0, pg.Offset)
	assert.Equal(t, 2, pg.Count)
	assert.Nil(t, pg.NextOffset)
}

func TestMetaAddsNextOffsetOnFullPage(t *testing.T) {
	pg := New(500, 3, MaxLimit)

	assert.Equal(t, MaxLimit, pg.Limit)
	assert.Equal(t, 3, pg.Offset)
	assert.Equal(t, MaxLimit, pg.Count)
	if assert.NotNil(t, pg.NextOffset) {
		assert.Equal(t, 103, *pg.NextOffset)
	}
}
