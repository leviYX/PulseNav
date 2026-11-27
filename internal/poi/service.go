package poi

import (
	"context"
	"time"

	"pulsenav/internal/domain"
)

// Repository 只接收业务层已校验的查询；驱动错误由适配器转换为领域错误。
type Repository interface {
	SearchPOI(context.Context, *domain.SearchRequest) ([]*domain.POI, error)
	Source() string
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }
func (s *Service) Source() string               { return s.repository.Source() }
func (s *Service) SearchPOI(ctx context.Context, req *domain.SearchRequest) ([]*domain.POI, error) {
	q, err := req.Normalize()
	if err != nil {
		return nil, err
	}
	// WithTimeout 不会延长上游更短的 deadline，数据库驱动接收相同的取消链。
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.repository.SearchPOI(ctx, q)
}
