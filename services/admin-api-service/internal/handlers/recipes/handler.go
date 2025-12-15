// Package recipes provides HTTP handlers for recipe management operations.
// Implements the /recipes/* endpoints per specs/025-model-recipes/plan.md
package recipes

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler handles recipe management HTTP requests
type Handler struct {
	service Service
}

// NewHandler creates a new recipes handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the recipe management routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/recipes", h.ListRecipes)
	r.Post("/recipes", h.CreateRecipe)
	r.Get("/recipes/{name}", h.GetRecipe)
	r.Put("/recipes/{name}", h.UpdateRecipe)
	r.Delete("/recipes/{name}", h.DeleteRecipe)
	r.Post("/recipes/{name}/validate", h.ValidateRecipe)
	r.Get("/recipes/{name}/deployments", h.ListRecipeDeployments)
}

// ListRecipes handles GET /recipes
func (h *Handler) ListRecipes(w http.ResponseWriter, r *http.Request) {
	opts := ListRecipesOptions{
		Runtime: r.URL.Query().Get("runtime"),
	}

	recipes, err := h.service.ListRecipes(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list recipes", err)
		return
	}

	// Convert to response format
	response := ListRecipesResponse{
		Recipes: make([]RecipeResponse, len(recipes)),
	}
	for i, recipe := range recipes {
		response.Recipes[i] = RecipeResponse{
			Name:        recipe.Name,
			DisplayName: recipe.DisplayName,
			Description: recipe.Description,
			ModelID:     recipe.ModelID,
			Runtime:     recipe.Runtime,
			Spec:        recipe.Spec,
			CreatedAt:   recipe.CreatedAt,
			UpdatedAt:   recipe.UpdatedAt,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// GetRecipe handles GET /recipes/:name
func (h *Handler) GetRecipe(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	recipe, err := h.service.GetRecipe(name)
	if err != nil {
		if errors.Is(err, ErrRecipeNotFound) || err.Error() == "recipe not found" {
			writeError(w, http.StatusNotFound, "recipe not found", err)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to get recipe", err)
		}
		return
	}

	// Convert to response format
	response := RecipeResponse{
		Name:        recipe.Name,
		DisplayName: recipe.DisplayName,
		Description: recipe.Description,
		ModelID:     recipe.ModelID,
		Runtime:     recipe.Runtime,
		Spec:        recipe.Spec,
		CreatedAt:   recipe.CreatedAt,
		UpdatedAt:   recipe.UpdatedAt,
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateRecipe handles POST /recipes
func (h *Handler) CreateRecipe(w http.ResponseWriter, r *http.Request) {
	var req CreateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	recipe, err := h.service.CreateRecipe(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create recipe", err)
		return
	}

	// Convert to response format
	response := RecipeResponse{
		Name:        recipe.Name,
		DisplayName: recipe.DisplayName,
		Description: recipe.Description,
		ModelID:     recipe.ModelID,
		Runtime:     recipe.Runtime,
		Spec:        recipe.Spec,
		CreatedAt:   recipe.CreatedAt,
		UpdatedAt:   recipe.UpdatedAt,
	}

	writeJSON(w, http.StatusCreated, response)
}

// UpdateRecipe handles PUT /recipes/:name
func (h *Handler) UpdateRecipe(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req UpdateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	recipe, err := h.service.UpdateRecipe(name, req)
	if err != nil {
		if errors.Is(err, ErrRecipeNotFound) || err.Error() == "recipe not found" {
			writeError(w, http.StatusNotFound, "recipe not found", err)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to update recipe", err)
		}
		return
	}

	// Convert to response format
	response := RecipeResponse{
		Name:        recipe.Name,
		DisplayName: recipe.DisplayName,
		Description: recipe.Description,
		ModelID:     recipe.ModelID,
		Runtime:     recipe.Runtime,
		Spec:        recipe.Spec,
		CreatedAt:   recipe.CreatedAt,
		UpdatedAt:   recipe.UpdatedAt,
	}

	writeJSON(w, http.StatusOK, response)
}

// DeleteRecipe handles DELETE /recipes/:name
func (h *Handler) DeleteRecipe(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := h.service.DeleteRecipe(name); err != nil {
		if errors.Is(err, ErrRecipeNotFound) || err.Error() == "recipe not found" {
			writeError(w, http.StatusNotFound, "recipe not found", err)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to delete recipe", err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ValidateRecipe handles POST /recipes/:name/validate
func (h *Handler) ValidateRecipe(w http.ResponseWriter, r *http.Request) {
	var req ValidateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	result, err := h.service.ValidateRecipe(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate recipe", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListRecipeDeployments handles GET /recipes/:name/deployments
func (h *Handler) ListRecipeDeployments(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	deployments, err := h.service.ListRecipeDeployments(name)
	if err != nil {
		if errors.Is(err, ErrRecipeNotFound) || err.Error() == "recipe not found" {
			writeError(w, http.StatusNotFound, "recipe not found", err)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to list deployments", err)
		}
		return
	}

	response := ListDeploymentsResponse{
		Deployments: deployments,
	}

	writeJSON(w, http.StatusOK, response)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"details": err.Error(),
	})
}
