package http

import (
	"errors"
	"log/slog"
	"net/http"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

// handlerDeleteAd serves DELETE /ads/{adID}. Only the owner of the ad can delete it.
func handlerDeleteAd(deleteAd usecase.DeleteAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		err = deleteAd(r.Context(), usecase.DeleteAdRequest{
			ID:          adID,
			RequesterID: middleware.UserIDFromRequest(r),
		})

		switch {
		case err == nil:
			rw.WriteHeader(http.StatusNoContent)
		case errors.Is(err, adsdao.ErrNotFound):
			rw.WriteHeader(http.StatusNotFound)
		case errors.Is(err, usecase.ErrForbidden):
			rw.WriteHeader(http.StatusForbidden)
		default:
			slog.ErrorContext(r.Context(), "unable to delete ad", "ad_id", adID, "error", err)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}
}
