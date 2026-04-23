package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"plexplore/internal/store"
)

type userResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type listUsersResponse struct {
	Users []userResponse `json:"users"`
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

func registerUserRoutes(mux *http.ServeMux, userStore UserStore, sessionStore SessionStore) {
	authAdmin := func(next http.Handler) http.Handler {
		return LoadCurrentUserFromSession(
			sessionStore,
			userStore,
			RequireUserSessionAuth(RequireAdmin(next)),
		)
	}

	mux.Handle("GET /api/v1/users", authAdmin(http.HandlerFunc(listUsersHandler(userStore))))
	mux.Handle("POST /api/v1/users", authAdmin(http.HandlerFunc(createUserHandler(userStore))))
}

func listUsersHandler(userStore UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := userStore.ListUsers(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list users failed")
			return
		}

		out := make([]userResponse, 0, len(users))
		for _, user := range users {
			out = append(out, toUserResponse(user))
		}
		writeJSON(w, http.StatusOK, listUsersResponse{Users: out})
	}
}

func createUserHandler(userStore UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateCSRF(r) {
			writeJSONError(w, http.StatusForbidden, "csrf token invalid")
			return
		}

		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		email := strings.TrimSpace(req.Email)
		password := strings.TrimSpace(req.Password)
		if email == "" || password == "" {
			writeJSONError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		hash, err := HashPassword(password)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		created, err := userStore.CreateUser(r.Context(), store.CreateUserParams{
			Name:         strings.TrimSpace(req.Name),
			Email:        email,
			PasswordHash: hash,
			IsAdmin:      req.IsAdmin,
		})
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "create user failed")
			return
		}
		writeJSON(w, http.StatusCreated, toUserResponse(created))
	}
}

func toUserResponse(user store.User) userResponse {
	return userResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		IsAdmin:   user.IsAdmin,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: user.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
