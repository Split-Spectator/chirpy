package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"errors"

	//"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) postChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
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

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

	const maxChirpLength = 140
	if len(params.Body) > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long", nil)
		return
	}

	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	cleaned := getCleanedBody(params.Body, badWords)

	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   cleaned,
		UserID: userID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create chirp", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
	})
}

func getCleanedBody(body string, badWords map[string]struct{}) string {
	words := strings.Split(body, " ")
	for i, word := range words {
		loweredWord := strings.ToLower(word)
		if _, ok := badWords[loweredWord]; ok {
			words[i] = "****"
		}
	}
	cleaned := strings.Join(words, " ")
	return cleaned
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	authorID := r.URL.Query().Get("author_id")
	var chirpsfromDB []database.Chirp
	var err error

	if authorID != "" {
		id, parseErr := uuid.Parse(authorID)
		if parseErr != nil {
			respondWithError(w, http.StatusBadRequest, "invalid author_id", parseErr)
			return
		}
		chirpsfromDB, err = cfg.db.GetChirpsByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			respondWithError(w, http.StatusInternalServerError, "error fetching chirps", err)
			return
		}
	} else {
		chirpsfromDB, err = cfg.db.GetAllChirps(r.Context())
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error fetching chirps", err)
			return
		}
	}
	respondWithJSON(w, http.StatusOK, chirpModelsToAPIChirps(chirpsfromDB))
}

func chirpModelsToAPIChirps(models []database.Chirp) []Chirp {
	apiChirps := make([]Chirp, 0, len(models))
	for _, m := range models {
		apiChirps = append(apiChirps, Chirp{
			ID:        m.ID,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			Body:      m.Body,
			UserId:    m.UserID,
		})
	}
	return apiChirps
}

func (cfg *apiConfig) getOneChirpHandler(w http.ResponseWriter, r *http.Request) {
	parsedID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Chirp ID", err)
	}

	oneChirp, err := cfg.db.GetOneChirps(r.Context(), parsedID)
	if errors.Is(err, sql.ErrNoRows) {
		respondWithError(w, http.StatusNotFound, "ID not Found", err)
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error fetching chirp by ID", err)
		return
	}

	respondWithJSON(w, http.StatusOK, Chirp{
		ID:        oneChirp.ID,
		CreatedAt: oneChirp.CreatedAt,
		UpdatedAt: oneChirp.UpdatedAt,
		Body:      oneChirp.Body,
		UserId:    oneChirp.UserID,
	})

}

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {

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
	chirpID := r.PathValue("chirpID")
	id, err := uuid.Parse(chirpID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chirp, err := cfg.db.GetOneChirps(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if userID != chirp.UserID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = cfg.db.DeleteChirp(r.Context(), id)
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
