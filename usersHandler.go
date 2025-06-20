package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "error decoding request", nil)
		return
	}
	user, err := cfg.db.CreateUser(r.Context(), params.Email)
	if err != nil {
		fmt.Println(err)
		respondWithError(w, 500, "Failed to create user", nil)
		return
	}

	respondWithJSON(w, 201, User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}
