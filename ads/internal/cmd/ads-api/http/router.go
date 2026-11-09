package http

import (
	"fmt"
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

// maxBodyBytes caps request bodies. The largest legitimate one is a full import:
// 1000 ads with long titles and photo URLs stay well below this limit.
const maxBodyBytes = 4 << 20

// Usecases groups the business functions the HTTP layer depends on.
type Usecases struct {
	ListAds   usecase.ListAdsFunc
	GetAd     usecase.GetAdByIDFunc
	CreateAd  usecase.CreateAdFunc
	DeleteAd  usecase.DeleteAdFunc
	ImportAds usecase.ImportAdsFunc
}

// validate fails when a usecase is missing: without it, the route would compile and then panic on its first request.
func (u Usecases) validate() error {
	missing := map[string]bool{
		"ListAds":   u.ListAds == nil,
		"GetAd":     u.GetAd == nil,
		"CreateAd":  u.CreateAd == nil,
		"DeleteAd":  u.DeleteAd == nil,
		"ImportAds": u.ImportAds == nil,
	}

	for name, isMissing := range missing {
		if isMissing {
			return fmt.Errorf("usecase %s is not wired", name)
		}
	}

	return nil
}

// NewRouter wires the routes of the API. It fails at startup when a usecase is missing.
func NewRouter(usecases Usecases) (http.Handler, error) {
	if err := usecases.validate(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	mux.Handle("GET /ads", handlerListAds(usecases.ListAds))
	mux.Handle("GET /ads/{adID}", handlerGetAd(usecases.GetAd))
	mux.Handle("POST /ads", middleware.RequireAuth(handlerCreateAd(usecases.CreateAd)))
	mux.Handle("DELETE /ads/{adID}", middleware.RequireAuth(handlerDeleteAd(usecases.DeleteAd)))
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(usecases.ImportAds)))

	mux.HandleFunc("GET /health", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	return middleware.Recover(http.MaxBytesHandler(mux, maxBodyBytes)), nil
}
