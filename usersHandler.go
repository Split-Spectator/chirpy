package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error decoding request", err)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "hashing password failed", err)
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Creating User failed", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

func (cfg *apiConfig) usersLoginHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password         string `json:"password"`
		Email            string `json:"email"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}
	type response struct {
		User
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = auth.CheckPasswordHash(user.HashedPassword, params.Password)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	refreshTokenString, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create refresh token", err)
		return
	}

	refreshTokenExpiry := time.Now().Add(60 * 24 * time.Hour)
	_, err = cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshTokenString,
		UserID:    user.ID,
		ExpiresAt: refreshTokenExpiry,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't store refresh token", err)
		return
	}

	accessToken, err := auth.MakeJWT(
		user.ID,
		cfg.secret,
		1*time.Hour,
	)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create access JWT", err)
		return
	}

	respondWithJSON(w, http.StatusOK, response{
		User: User{
			ID:          user.ID,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
			Email:       user.Email,
			IsChirpyRed: user.IsChirpyRed,
		},
		Token:        accessToken,
		RefreshToken: refreshTokenString,
	})
}

func (cfg *apiConfig) createRefreshToken(c context.Context, userID uuid.UUID) (string, error) {
	const refreshTokenValidity = 60 * (24 * time.Hour)
	rTokenString, err := auth.MakeRefreshToken()
	if err != nil {
		return "", err
	}
	params := database.CreateRefreshTokenParams{
		Token:     rTokenString,
		UserID:    userID,
		ExpiresAt: time.Now().Add(refreshTokenValidity),
	}
	_, err = cfg.db.CreateRefreshToken(c, params)
	if err != nil {
		return "", err
	}
	return rTokenString, nil
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	rTokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	refreshToken, err := cfg.db.GetRefreshToken(r.Context(), rTokenString)
	if err != nil || !isRefreshTokenValid(refreshToken) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), rTokenString)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	jwtTokenString, err := auth.MakeJWT(user.ID.UUID, cfg.secret, 1*time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	type RespBody struct {
		Token string `json:"token"`
	}
	respBody := RespBody{Token: jwtTokenString}

	respondWithJSON(w, http.StatusOK, respBody)
}

func isExpired(expiresAt time.Time) bool {

	return expiresAt.Compare(time.Now()) == -1
}

func isRefreshTokenValid(refreshToken database.RefreshToken) bool {
	return !(isExpired(refreshToken.ExpiresAt) || refreshToken.RevokedAt.Valid)
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	rTokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = cfg.db.Revoke(r.Context(), rTokenString)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// To prevent token probing
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	params := parameters{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters", err)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "hashing password failed", err)
		return
	}

	user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
		HashedPassword: hashedPassword,
		Email:          params.Email,
		ID:             userID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating user", err)
		return
	}

	respondWithJSON(w, http.StatusOK, User{
		ID:          userID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})

}

func (cfg *apiConfig) handlerMakeRed(w http.ResponseWriter, r *http.Request) {
	apiKey, err := auth.GetAPIKey(r.Header)
	if err != nil || apiKey != cfg.polkaKey {
		w.WriteHeader(401)
		return
	}

	type DataStruct struct {
		UserID string `json:"user_id"`
	}
	type ExpectedReq struct {
		Event string     `json:"event"`
		Data  DataStruct `json:"data"`
	}
	var expectedReq ExpectedReq
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&expectedReq)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	const userUpdgraded = "user.upgraded"
	if expectedReq.Event != userUpdgraded {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	id, err := uuid.Parse(expectedReq.Data.UserID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = cfg.db.UpgradeUserRed(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
