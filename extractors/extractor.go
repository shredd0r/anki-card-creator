package extractors

import "context"

type CardExtractor[TYPE_OF_CARD any] interface {
	GetLength(ctx context.Context) (*int, error)
	GetLessons(ctx context.Context) (*[]string, error)
	GetCards(ctx context.Context, index int) (*[]*TYPE_OF_CARD, error)
}
