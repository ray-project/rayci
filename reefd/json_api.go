package reefd

import (
	"context"
	"encoding/json"
	"net/http"
)

func jsonAPI[R, S any](f func(ctx context.Context, req *R) (*S, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("Accept") != "application/json" {
			http.Error(w, "must use application/json", http.StatusUnsupportedMediaType)
			return
		}

		const jsonContentType = "application/json"
		if r.Header.Get("Content-Type") != jsonContentType {
			http.Error(w, "must use application/json", http.StatusUnsupportedMediaType)
		}

		req := new(R)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if dec.More() {
			http.Error(w, "extra data after request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		ctx := r.Context()
		resp, err := f(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", jsonContentType)

		enc := json.NewEncoder(w)
		if err := enc.Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
