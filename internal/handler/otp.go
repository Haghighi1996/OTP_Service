package handler

import (
	"encoding/json"
	"net/http"
)

type sendOTPRequest struct {
	PhoneNumber string `json:"phone_number"`
}

func SendOTPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req sendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"phone_number": req.PhoneNumber})

}
