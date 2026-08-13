package reqenc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func (s *Service) ConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, Response{Code: 200, Data: s.ConfigData()})
	}
}

func (s *Service) SessionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.Mode == ModeDisabled {
			writeError(w, ErrDisabled)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 16<<10))
		if err != nil {
			writeError(w, ErrInvalidPayload)
			return
		}
		var req CreateSessionRequest
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, ErrInvalidPayload)
			return
		}
		data, err := s.CreateSession(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, Response{Code: 200, Data: data})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
