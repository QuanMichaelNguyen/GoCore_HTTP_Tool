package utils

import (
	"encoding/json"
	"fmt"
	"go-server/models"
	"net/http"
	"strconv"
)

type ResponseWithMeta struct {
	Post           models.Post `json:"post"`
	Source         string      `json:"source"`
	ResponseTimeMs int64       `json:"responseTimeMs"`
}

// Utility helpers
func RespondWithJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func RespondWithStatus(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func RespondWithMetadata(w http.ResponseWriter, post models.Post, source string, duration int64, fromCache bool) {
	if fromCache {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	w.Header().Set("X-Response-Time-Ms", fmt.Sprintf("%d", duration))
	RespondWithJSON(w, ResponseWithMeta{Post: post, Source: source, ResponseTimeMs: duration})
}

func ParsePaginationParams(r *http.Request) (limit, offset int) {
	limit, offset = 10, 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return
}
