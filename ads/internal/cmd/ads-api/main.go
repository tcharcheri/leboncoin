package main

import (
	"context"
	"log/slog"
	"os"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/http"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usercache"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/httpserver"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg := Load()

	adDAO, err := postgres.New(ctx, cfg.DatabaseURL, cfg.DBMaxOpenConns)
	if err != nil {
		return err
	}
	defer func() {
		if err := adDAO.Close(); err != nil {
			slog.Error("unable to close the database", "error", err)
		}
	}()

	users := usercache.New(usersapi.NewClient(cfg.IdentityURL, cfg.IdentityTimeout), cfg.UserCacheTTL, cfg.UserCacheSize)

	router, err := http.NewRouter(http.Usecases{
		CreateAd:  usecase.NewCreateAdFunc(adDAO),
		DeleteAd:  usecase.NewDeleteAdFunc(adDAO),
		GetAd:     usecase.NewGetAdByIDFunc(adDAO, users),
		ImportAds: usecase.NewImportAdsFunc(adDAO),
		ListAds:   usecase.NewListAdsFunc(adDAO),
	})
	if err != nil {
		return err
	}

	return httpserver.Run(ctx, cfg.HTTPAddr, router)
}
