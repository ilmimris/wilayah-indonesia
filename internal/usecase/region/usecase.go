package region

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ilmimris/wilayah-indonesia/internal/entity"
	"github.com/ilmimris/wilayah-indonesia/internal/model"
	repository "github.com/ilmimris/wilayah-indonesia/internal/repository"
	sharederrors "github.com/ilmimris/wilayah-indonesia/internal/shared/errors"
	regionmatcher "github.com/ilmimris/wilayah-indonesia/internal/usecase/region/matcher"
	"github.com/ilmimris/wilayah-indonesia/internal/usecase/shared"
	"github.com/ilmimris/wilayah-indonesia/pkg/regionhierarchy"
)

// RegionUseCase exposes search operations across administrative regions.
type RegionUseCase interface {
	Search(ctx context.Context, req model.SearchRequest) (model.SearchResponse, error)
	SearchByDistrict(ctx context.Context, district, city, province string, opts model.SearchOptions) (model.SearchResponse, error)
	SearchBySubdistrict(ctx context.Context, subdistrict, district, city, province string, opts model.SearchOptions) (model.SearchResponse, error)
	SearchByCity(ctx context.Context, city string, opts model.SearchOptions) (model.SearchResponse, error)
	SearchByProvince(ctx context.Context, province string, opts model.SearchOptions) (model.SearchResponse, error)
	SearchByPostalCode(ctx context.Context, postalCode string, opts model.SearchOptions) (model.SearchResponse, error)
}

// RegionUseCaseOptions configures behaviour of the use case implementation.
type RegionUseCaseOptions struct {
	Logger          *slog.Logger
	DefaultLimit    int
	MaxLimit        int
	Matcher         *regionmatcher.Matcher
	MatcherMinScore float64
}

type regionUseCase struct {
	repo            repository.RegionRepository
	normalizer      shared.OptionNormalizer
	logger          *slog.Logger
	capabilities    repository.RegionRepositoryCapabilities
	matcher         *regionmatcher.Matcher
	matcherMinScore float64
}

// New creates a RegionUseCase backed by the provided repository.
// It retrieves repository capabilities and returns an error if that fails.
// If ctx is nil, context.Background() is used. If opts.Logger is nil, slog.Default() is used.
// MatcherMinScore defaults to 0.6 when opts.MatcherMinScore is zero or negative.
// The returned use case is configured with the provided repository, a LimitNormalizer
// New creates a RegionUseCase wired to the given repository and configured by opts.
// It queries the repository for capabilities (returning an error if that call fails),
// uses opts.Logger or slog.Default() when nil, and constructs a limit normalizer from
// opts.DefaultLimit and opts.MaxLimit. If opts.MatcherMinScore is zero or negative,
// a default min score of 0.6 is used. The returned use case will include opts.Matcher
// and the resolved matcher minimum score.
func New(ctx context.Context, repo repository.RegionRepository, opts RegionUseCaseOptions) (RegionUseCase, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	caps, err := repo.Capabilities(ctx)
	if err != nil {
		return nil, err
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	minScore := opts.MatcherMinScore
	if minScore <= 0 {
		minScore = 0.6
	}

	return &regionUseCase{
		repo:            repo,
		normalizer:      shared.LimitNormalizer{Default: opts.DefaultLimit, Max: opts.MaxLimit},
		logger:          logger,
		capabilities:    caps,
		matcher:         opts.Matcher,
		matcherMinScore: minScore,
	}, nil
}

func (uc *regionUseCase) Search(ctx context.Context, req model.SearchRequest) (model.SearchResponse, error) {
	sanitizedQuery := sanitizeFTSQuery(req.Query)
	trimmedQuery := strings.TrimSpace(sanitizedQuery)

	if trimmedQuery == "" && req.Subdistrict == "" && req.District == "" && req.City == "" && req.Province == "" {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "at least one search parameter is required")
	}

	if err := uc.normalizer.Normalize(&req.Options); err != nil {
		return model.SearchResponse{}, err
	}

	if err := uc.validateDatasetOptions(req.Options); err != nil {
		return model.SearchResponse{}, err
	}

	req.Query = trimmedQuery
	suggestion := uc.applySuggestion(req.Query, &req)

	params := repository.RegionSearchParams{
		Query:       req.Query,
		Subdistrict: req.Subdistrict,
		District:    req.District,
		City:        req.City,
		Province:    req.Province,
		Options: repository.RegionSearchOptions{
			Limit:         req.Options.Limit,
			SearchBPS:     req.Options.SearchBPS,
			IncludeBPS:    req.Options.IncludeBPS,
			IncludeScores: req.Options.IncludeScores,
		},
	}

	results, err := uc.repo.Search(ctx, params)
	if err != nil {
		return model.SearchResponse{}, err
	}

	return model.SearchResponse{Items: mapToResponses(results), Suggestion: suggestion}, nil
}

func (uc *regionUseCase) applySuggestion(query string, req *model.SearchRequest) *model.Suggestion {
	if uc.matcher == nil {
		return nil
	}
	if strings.TrimSpace(query) == "" {
		return nil
	}
	suggestion := uc.matcher.Suggest(query)
	modelSuggestion := convertSuggestion(suggestion)
	if suggestion.Strategy == "percolator" && suggestion.Score >= uc.matcherMinScore {
		uc.applyHierarchicalSuggestions(suggestion, req)
	}
	return modelSuggestion
}

func (uc *regionUseCase) applyHierarchicalSuggestions(s regionmatcher.Suggestion, req *model.SearchRequest) {
	// Apply province suggestion
	if req.Province == "" {
		req.Province = uc.extractProvinceName(s)
	}
	// Apply city suggestion
	if req.City == "" {
		req.City = uc.extractCityName(s)
	}
	// Apply district suggestion
	if req.District == "" {
		req.District = uc.extractDistrictName(s)
	}
	// Apply subdistrict suggestion
	if req.Subdistrict == "" && s.Subdistrict != nil {
		req.Subdistrict = s.Subdistrict.Name
	}
}

func (uc *regionUseCase) extractProvinceName(s regionmatcher.Suggestion) string {
	if s.Province != nil && s.Province.Name != "" {
		return s.Province.Name
	}
	if s.City != nil && s.City.Province != "" {
		return s.City.Province
	}
	if s.District != nil && s.District.Province != "" {
		return s.District.Province
	}
	if s.Subdistrict != nil && s.Subdistrict.Province != "" {
		return s.Subdistrict.Province
	}
	return ""
}

func (uc *regionUseCase) extractCityName(s regionmatcher.Suggestion) string {
	if s.City != nil && s.City.Name != "" {
		return s.City.Name
	}
	if s.District != nil && s.District.City != "" {
		return s.District.City
	}
	if s.Subdistrict != nil && s.Subdistrict.City != "" {
		return s.Subdistrict.City
	}
	return ""
}

func (uc *regionUseCase) extractDistrictName(s regionmatcher.Suggestion) string {
	if s.District != nil && s.District.Name != "" {
		return s.District.Name
	}
	if s.Subdistrict != nil && s.Subdistrict.District != "" {
		return s.Subdistrict.District
	}
	return ""
}

func (uc *regionUseCase) SearchByDistrict(ctx context.Context, district, city, province string, opts model.SearchOptions) (model.SearchResponse, error) {
	if district == "" {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "query parameter is required")
	}
	return uc.Search(ctx, model.SearchRequest{District: district, City: city, Province: province, Options: opts})
}

func (uc *regionUseCase) SearchBySubdistrict(ctx context.Context, subdistrict, district, city, province string, opts model.SearchOptions) (model.SearchResponse, error) {
	if subdistrict == "" {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "query parameter is required")
	}
	return uc.Search(ctx, model.SearchRequest{Subdistrict: subdistrict, District: district, City: city, Province: province, Options: opts})
}

func (uc *regionUseCase) SearchByCity(ctx context.Context, city string, opts model.SearchOptions) (model.SearchResponse, error) {
	if city == "" {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "query parameter is required")
	}
	return uc.Search(ctx, model.SearchRequest{City: city, Options: opts})
}

func (uc *regionUseCase) SearchByProvince(ctx context.Context, province string, opts model.SearchOptions) (model.SearchResponse, error) {
	if province == "" {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "query parameter is required")
	}
	return uc.Search(ctx, model.SearchRequest{Province: province, Options: opts})
}

func (uc *regionUseCase) SearchByPostalCode(ctx context.Context, postalCode string, opts model.SearchOptions) (model.SearchResponse, error) {
	if postalCode == "" {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "postal code parameter is required")
	}

	if err := uc.normalizer.Normalize(&opts); err != nil {
		return model.SearchResponse{}, err
	}
	if opts.IncludeBPS && !uc.capabilities.HasBPSColumns {
		return model.SearchResponse{}, sharederrors.New(sharederrors.CodeInvalidInput, "BPS metadata requested but dataset is missing BPS columns; run 'make prepare-db'")
	}

	results, err := uc.repo.SearchByPostalCode(ctx, postalCode, repository.RegionSearchOptions{
		Limit:         opts.Limit,
		SearchBPS:     opts.SearchBPS,
		IncludeBPS:    opts.IncludeBPS,
		IncludeScores: opts.IncludeScores,
	})
	if err != nil {
		return model.SearchResponse{}, err
	}

	return model.SearchResponse{Items: mapToResponses(results)}, nil
}

func (uc *regionUseCase) validateDatasetOptions(opts model.SearchOptions) error {
	if opts.SearchBPS && !uc.capabilities.HasBPSColumns {
		return sharederrors.New(sharederrors.CodeInvalidInput, "BPS search requested but dataset is missing BPS columns; run 'make prepare-db'")
	}
	if opts.IncludeBPS && !uc.capabilities.HasBPSColumns {
		return sharederrors.New(sharederrors.CodeInvalidInput, "BPS metadata requested but dataset is missing BPS columns; run 'make prepare-db'")
	}
	if opts.SearchBPS && !uc.capabilities.HasBPSIndex {
		return sharederrors.New(sharederrors.CodeInvalidInput, "BPS search requested but dataset is missing BPS FTS index; run 'make prepare-db'")
	}
	return nil
}

// mapToResponses converts a slice of entity.RegionWithScore into a slice of model.RegionResponse.
// Each resulting response contains the region fields (ID, Subdistrict, District, City, Province,
// mapToResponses converts a slice of entity.RegionWithScore to a slice of model.RegionResponse,
// copying ID, Subdistrict, District, City, Province, PostalCode, FullText and BPS, and sets
// Scores on the response when the source Score is non-nil.
func mapToResponses(items []entity.RegionWithScore) []model.RegionResponse {
	responses := make([]model.RegionResponse, 0, len(items))
	for _, item := range items {
		resp := model.RegionResponse{
			ID:          item.Region.ID,
			Subdistrict: item.Region.Subdistrict,
			District:    item.Region.District,
			City:        item.Region.City,
			Province:    item.Region.Province,
			PostalCode:  item.Region.PostalCode,
			FullText:    item.Region.FullText,
			BPS:         item.Region.BPS,
		}
		if item.Score != nil {
			resp.Scores = item.Score
		}
		responses = append(responses, resp)
	}
	return responses
}

// convertSuggestion converts a regionmatcher.Suggestion into a model.Suggestion suitable for API responses.
// If the input has no province/city/district/subdistrict matches it returns nil.
// The result copies strategy, score and any provided matches; when a higher-level match (province, city, or district)
// is missing it attempts to derive it from available lower-level matches by resolving the corresponding region code
// convertSuggestion converts a regionmatcher.Suggestion into a *model.Suggestion suitable for API responses.
// If the input contains no province/city/district/subdistrict matches it returns nil.
// When a higher-level match (province, city, district) is missing, it attempts to infer it from available lower-level
// matches by resolving their region codes via the region hierarchy.
func convertSuggestion(s regionmatcher.Suggestion) *model.Suggestion {
	if s.Province == nil && s.City == nil && s.District == nil && s.Subdistrict == nil {
		return nil
	}
	toMatch := func(m *regionmatcher.Match) *model.SuggestedMatch {
		if m == nil {
			return nil
		}
		return &model.SuggestedMatch{
			Name:       m.Name,
			RegionID:   m.RegionID,
			Similarity: m.Similarity,
			Fragment:   m.Fragment,
		}
	}
	suggestion := &model.Suggestion{
		Strategy:    s.Strategy,
		Score:       s.Score,
		Province:    toMatch(s.Province),
		City:        toMatch(s.City),
		District:    toMatch(s.District),
		Subdistrict: toMatch(s.Subdistrict),
	}
	if suggestion.Province == nil {
		switch {
		case s.City != nil && s.City.Province != "":
			if code, err := regionhierarchy.CodeAtLevel(s.City.RegionID, regionhierarchy.LevelProvince); err == nil && code != "" {
				suggestion.Province = &model.SuggestedMatch{Name: s.City.Province, RegionID: code, Similarity: s.City.Similarity}
			}
		case s.District != nil && s.District.Province != "":
			if code, err := regionhierarchy.CodeAtLevel(s.District.RegionID, regionhierarchy.LevelProvince); err == nil && code != "" {
				suggestion.Province = &model.SuggestedMatch{Name: s.District.Province, RegionID: code, Similarity: s.District.Similarity}
			}
		case s.Subdistrict != nil && s.Subdistrict.Province != "":
			if code, err := regionhierarchy.CodeAtLevel(s.Subdistrict.RegionID, regionhierarchy.LevelProvince); err == nil && code != "" {
				suggestion.Province = &model.SuggestedMatch{Name: s.Subdistrict.Province, RegionID: code, Similarity: s.Subdistrict.Similarity}
			}
		}
	}
	if suggestion.City == nil {
		switch {
		case s.District != nil && s.District.City != "":
			if code, err := regionhierarchy.CodeAtLevel(s.District.RegionID, regionhierarchy.LevelCity); err == nil && code != "" {
				suggestion.City = &model.SuggestedMatch{Name: s.District.City, RegionID: code, Similarity: s.District.Similarity}
			}
		case s.Subdistrict != nil && s.Subdistrict.City != "":
			if code, err := regionhierarchy.CodeAtLevel(s.Subdistrict.RegionID, regionhierarchy.LevelCity); err == nil && code != "" {
				suggestion.City = &model.SuggestedMatch{Name: s.Subdistrict.City, RegionID: code, Similarity: s.Subdistrict.Similarity}
			}
		}
	}
	if suggestion.District == nil {
		if s.Subdistrict != nil && s.Subdistrict.District != "" {
			if code, err := regionhierarchy.CodeAtLevel(s.Subdistrict.RegionID, regionhierarchy.LevelDistrict); err == nil && code != "" {
				suggestion.District = &model.SuggestedMatch{Name: s.Subdistrict.District, RegionID: code, Similarity: s.Subdistrict.Similarity}
			}
		}
	}
	return suggestion
}

// sanitizeFTSQuery removes single-quote and double-quote characters from q.
// sanitizeFTSQuery removes all single-quote (') and double-quote (") characters from the input string.
// It is used to clean user-provided full-text search queries before they are processed by the search layer.
func sanitizeFTSQuery(q string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\'', '"':
			return -1
		default:
			return r
		}
	}, q)
}
