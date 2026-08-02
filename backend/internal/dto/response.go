package dto

type Meta struct {
	Page     int   `json:"page,omitempty"`
	PageSize int   `json:"page_size,omitempty"`
	Total    int64 `json:"total"`
	HasMore  bool  `json:"has_more,omitempty"`
}

type Response[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
}

func OK[T any](data T) *Response[T] {
	return &Response[T]{Data: data}
}

func Created[T any](data T) *Response[T] {
	return &Response[T]{Data: data}
}

func WithMessage[T any](data T, message string) *Response[T] {
	return &Response[T]{Data: data, Message: message}
}

func List[T any](items []T, total int64, page, pageSize int) *Response[[]T] {
	hasMore := page > 0 && pageSize > 0 && int64(page*pageSize) < total
	return &Response[[]T]{
		Data: items,
		Meta: &Meta{Page: page, PageSize: pageSize, Total: total, HasMore: hasMore},
	}
}
