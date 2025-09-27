package region

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ilmimris/wilayah-indonesia/internal/model"
	sharederrors "github.com/ilmimris/wilayah-indonesia/internal/shared/errors"
	regionusecase "github.com/ilmimris/wilayah-indonesia/internal/usecase/region"
)

type stubUseCase struct {
	response  model.SearchResponse
	returnErr error
	lastReq   model.SearchRequest
}

func (s *stubUseCase) Search(ctx context.Context, req model.SearchRequest) (model.SearchResponse, error) {
	s.lastReq = req
	return s.response, s.returnErr
}

func (s *stubUseCase) SearchByDistrict(ctx context.Context, district, city, province string, opts model.SearchOptions) (model.SearchResponse, error) {
	return s.Search(ctx, model.SearchRequest{District: district, City: city, Province: province, Options: opts})
}

func (s *stubUseCase) SearchBySubdistrict(ctx context.Context, subdistrict, district, city, province string, opts model.SearchOptions) (model.SearchResponse, error) {
	return s.Search(ctx, model.SearchRequest{Subdistrict: subdistrict, District: district, City: city, Province: province, Options: opts})
}

func (s *stubUseCase) SearchByCity(ctx context.Context, city string, opts model.SearchOptions) (model.SearchResponse, error) {
	return s.Search(ctx, model.SearchRequest{City: city, Options: opts})
}

func (s *stubUseCase) SearchByProvince(ctx context.Context, province string, opts model.SearchOptions) (model.SearchResponse, error) {
	return s.Search(ctx, model.SearchRequest{Province: province, Options: opts})
}

func (s *stubUseCase) SearchByPostalCode(ctx context.Context, postalCode string, opts model.SearchOptions) (model.SearchResponse, error) {
	return s.Search(ctx, model.SearchRequest{Options: opts})
}

func TestSearchHandlerSuccess(t *testing.T) {
	uc := &stubUseCase{response: model.SearchResponse{Items: []model.RegionResponse{{ID: "1", City: "Jakarta"}}}}
	controller := NewController(uc)
	app := fiber.New()
	controller.Register(app)

	req := httptest.NewRequest(http.MethodGet, "/search?q=jakarta&limit=5&include_bps=true", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if uc.lastReq.Options.Limit != 5 || !uc.lastReq.Options.IncludeBPS {
		t.Fatalf("unexpected options passed to use case: %+v", uc.lastReq.Options)
	}
}

func TestSearchHandlerInvalidBool(t *testing.T) {
	uc := &stubUseCase{}
	controller := NewController(uc)
	app := fiber.New()
	controller.Register(app)

	req := httptest.NewRequest(http.MethodGet, "/search?include_scores=maybe", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestSearchHandlerMapsUseCaseError(t *testing.T) {
	uc := &stubUseCase{returnErr: sharederrors.New(sharederrors.CodeInvalidInput, "bad input")}
	controller := NewController(uc)
	app := fiber.New()
	controller.Register(app)

	params := url.Values{}
	params.Set("q", "jakarta")
	req := httptest.NewRequest(http.MethodGet, "/search?"+params.Encode(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

var _ regionusecase.RegionUseCase = (*stubUseCase)(nil)
