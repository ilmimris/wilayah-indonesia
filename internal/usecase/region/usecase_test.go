package region

import (
	"context"
	"errors"
	"testing"

	"github.com/ilmimris/wilayah-indonesia/internal/entity"
	"github.com/ilmimris/wilayah-indonesia/internal/model"
	repository "github.com/ilmimris/wilayah-indonesia/internal/repository"
	sharederrors "github.com/ilmimris/wilayah-indonesia/internal/shared/errors"
	regionmatcher "github.com/ilmimris/wilayah-indonesia/internal/usecase/region/matcher"
)

type fakeRepository struct {
	items        []entity.RegionWithScore
	capabilities repository.RegionRepositoryCapabilities
	lastParams   repository.RegionSearchParams
	postalOpts   repository.RegionSearchOptions
	postalCode   string
	retErr       error
}

func (f *fakeRepository) Search(ctx context.Context, params repository.RegionSearchParams) ([]entity.RegionWithScore, error) {
	f.lastParams = params
	return f.items, f.retErr
}

func (f *fakeRepository) SearchByDistrict(ctx context.Context, params repository.RegionSearchParams) ([]entity.RegionWithScore, error) {
	return f.Search(ctx, params)
}

func (f *fakeRepository) SearchBySubdistrict(ctx context.Context, params repository.RegionSearchParams) ([]entity.RegionWithScore, error) {
	return f.Search(ctx, params)
}

func (f *fakeRepository) SearchByCity(ctx context.Context, params repository.RegionSearchParams) ([]entity.RegionWithScore, error) {
	return f.Search(ctx, params)
}

func (f *fakeRepository) SearchByProvince(ctx context.Context, params repository.RegionSearchParams) ([]entity.RegionWithScore, error) {
	return f.Search(ctx, params)
}

func (f *fakeRepository) SearchByPostalCode(ctx context.Context, postalCode string, opts repository.RegionSearchOptions) ([]entity.RegionWithScore, error) {
	f.postalCode = postalCode
	f.postalOpts = opts
	return f.items, f.retErr
}

func (f *fakeRepository) Capabilities(ctx context.Context) (repository.RegionRepositoryCapabilities, error) {
	return f.capabilities, nil
}

func TestSearchValidation(t *testing.T) {
	repo := &fakeRepository{}
	uc, err := New(context.Background(), repo, RegionUseCaseOptions{})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	_, err = uc.Search(context.Background(), model.SearchRequest{})
	if err == nil || !sharederrors.Is(err, sharederrors.CodeInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}

	_, err = uc.Search(context.Background(), model.SearchRequest{Query: "'"})
	if err == nil || !sharederrors.Is(err, sharederrors.CodeInvalidInput) {
		t.Fatalf("expected invalid input error for sanitized empty query, got %v", err)
	}
}

func TestSearchUsesRepository(t *testing.T) {
	repo := &fakeRepository{
		items: []entity.RegionWithScore{{
			Region: entity.Region{ID: "1", City: "Jakarta", Province: "DKI"},
			Score:  &entity.RegionScore{},
		}},
		capabilities: repository.RegionRepositoryCapabilities{HasBPSColumns: true, HasBPSIndex: true},
	}

	uc, err := New(context.Background(), repo, RegionUseCaseOptions{})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	resp, err := uc.Search(context.Background(), model.SearchRequest{
		Query:   "jakarta",
		Options: model.SearchOptions{IncludeBPS: true, IncludeScores: true},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].City != "Jakarta" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !repo.lastParams.Options.IncludeBPS || !repo.lastParams.Options.IncludeScores {
		t.Fatalf("repository options not propagated: %+v", repo.lastParams.Options)
	}
}

func TestSearchSanitizesFTSQuery(t *testing.T) {
	repo := &fakeRepository{capabilities: repository.RegionRepositoryCapabilities{HasBPSColumns: true, HasBPSIndex: true}}
	uc, err := New(context.Background(), repo, RegionUseCaseOptions{})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	if _, err := uc.Search(context.Background(), model.SearchRequest{Query: "'quoted'"}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if repo.lastParams.Query != "quoted" {
		t.Fatalf("expected sanitized query, got %q", repo.lastParams.Query)
	}
}

func TestPostalCodeValidation(t *testing.T) {
	repo := &fakeRepository{capabilities: repository.RegionRepositoryCapabilities{HasBPSColumns: true}}
	uc, err := New(context.Background(), repo, RegionUseCaseOptions{})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	_, err = uc.SearchByPostalCode(context.Background(), "", model.SearchOptions{})
	if err == nil || !sharederrors.Is(err, sharederrors.CodeInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestRepositoryErrorPropagation(t *testing.T) {
	repoErr := errors.New("boom")
	repo := &fakeRepository{retErr: repoErr}
	uc, err := New(context.Background(), repo, RegionUseCaseOptions{})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	_, err = uc.Search(context.Background(), model.SearchRequest{Query: "jakarta"})
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repo error to propagate, got %v", err)
	}
}

func TestSearchAppliesMatcherSuggestion(t *testing.T) {
	facets := []regionmatcher.Facet{{
		RegionID:    "11.01.01.2001",
		Subdistrict: "Keude Bakongan",
		District:    "Bakongan",
		City:        "Kabupaten Aceh Selatan",
		Province:    "Aceh",
	}}
	matcher, err := regionmatcher.NewMatcher(facets,
		regionmatcher.WithLevelThreshold(regionmatcher.LevelProvince, 0.2),
		regionmatcher.WithLevelThreshold(regionmatcher.LevelCity, 0.2),
		regionmatcher.WithLevelThreshold(regionmatcher.LevelDistrict, 0.2),
		regionmatcher.WithLevelThreshold(regionmatcher.LevelSubdistrict, 0.2),
	)
	if err != nil {
		t.Fatalf("failed to build matcher: %v", err)
	}
	repo := &fakeRepository{capabilities: repository.RegionRepositoryCapabilities{HasBPSColumns: true, HasBPSIndex: true}}
	uc, err := New(context.Background(), repo, RegionUseCaseOptions{Matcher: matcher, MatcherMinScore: 0.6})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	resp, err := uc.Search(context.Background(), model.SearchRequest{Query: "kabupaten aceh selatan bakongan keude bakongan"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if repo.lastParams.Province != "Aceh" || repo.lastParams.City != "Kabupaten Aceh Selatan" {
		t.Fatalf("expected repository params to include suggestion, got %+v", repo.lastParams)
	}
	if resp.Suggestion == nil || resp.Suggestion.Province == nil || resp.Suggestion.Province.Name != "Aceh" {
		t.Fatalf("expected suggestion to be returned, got %+v", resp.Suggestion)
	}
}

func TestSearchDoesNotApplySuggestionBelowThreshold(t *testing.T) {
	facets := []regionmatcher.Facet{{
		RegionID:    "11.01.01.2001",
		Subdistrict: "Keude Bakongan",
		District:    "Bakongan",
		City:        "Kabupaten Aceh Selatan",
		Province:    "Aceh",
	}}
	matcher, err := regionmatcher.NewMatcher(facets,
		regionmatcher.WithLevelThreshold(regionmatcher.LevelProvince, 0.2),
		regionmatcher.WithLevelThreshold(regionmatcher.LevelCity, 0.2),
		regionmatcher.WithLevelThreshold(regionmatcher.LevelDistrict, 0.2),
		regionmatcher.WithLevelThreshold(regionmatcher.LevelSubdistrict, 0.2),
	)
	if err != nil {
		t.Fatalf("failed to build matcher: %v", err)
	}
	repo := &fakeRepository{capabilities: repository.RegionRepositoryCapabilities{HasBPSColumns: true, HasBPSIndex: true}}
	uc, err := New(context.Background(), repo, RegionUseCaseOptions{Matcher: matcher, MatcherMinScore: 1.1})
	if err != nil {
		t.Fatalf("failed to construct use case: %v", err)
	}

	resp, err := uc.Search(context.Background(), model.SearchRequest{Query: "kabupaten aceh selatan bakongan keude bakongan"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if repo.lastParams.Province != "" || repo.lastParams.City != "" {
		t.Fatalf("expected repository params to remain unchanged, got %+v", repo.lastParams)
	}
	if resp.Suggestion == nil {
		t.Fatalf("expected suggestion even when not applied")
	}
}
