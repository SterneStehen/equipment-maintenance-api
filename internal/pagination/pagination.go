package pagination

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Meta struct {
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	Count      int  `json:"count"`
	NextOffset *int `json:"next_offset,omitempty"`
}

func Normalize(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func New(limit, offset, count int) Meta {
	limit, offset = Normalize(limit, offset)
	var next *int
	if count == limit {
		n := offset + limit
		next = &n
	}
	return Meta{Limit: limit, Offset: offset, Count: count, NextOffset: next}
}
