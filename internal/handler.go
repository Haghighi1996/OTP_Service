package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type handler struct {
	service *partService
}

func NewHandler(service *partService) *handler {
	return &handler{
		service: service,
	}
}

func registerRoutes(h *handler) {
	http.HandleFunc("/parts", h.GetAllParts)
	http.HandleFunc("/parts/create", h.CreatePart)
	http.HandleFunc("/parts/delete", h.DeletePart)
}

func (h *handler) GetAllParts(w http.ResponseWriter, r *http.Request) {
	parts := h.service.GetAllParts()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(parts); err != nil {
		http.Error(w, "Failed to encode parts", http.StatusInternalServerError)
		return
	}
	// Convert parts to JSON and write to response
}

func (h *handler) CreatePart(w http.ResponseWriter, r *http.Request) {

	var part Part
	if err := json.NewDecoder(r.Body).Decode(&part); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	createdPart := h.service.CreatePart(part.Name, part.Type, part.Weight, part.Quantity)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdPart); err != nil {
		http.Error(w, "Failed to encode created part", http.StatusInternalServerError)
		return
	}
}

func (h *handler) DeletePart(w http.ResponseWriter, r *http.Request) {
	idParam := r.URL.Query().Get("id")
	//idParam := r.URL.Query().Get("id")
	if idParam == "" {
		http.Error(w, "Missing part ID", http.StatusBadRequest)
		return
	}

	var id int64
	if _, err := fmt.Sscan(idParam, &id); err != nil {
		http.Error(w, "Invalid part ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeletePart(id); err != nil {
		http.Error(w, "Failed to delete part", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
