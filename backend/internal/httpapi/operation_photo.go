package httpapi

import (
	"bytes"
	"github.com/go-chi/chi/v5"
	"image/jpeg"
	"io"
	"net/http"
)

func (s *Server) listOperationPhotos(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	photos, err := s.repo.OperationPhotos(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, photos)
}
func (s *Server) getOperationPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	data, err := s.repo.OperationPhoto(r.Context(), id, chi.URLParam(r, "digest"))
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
func (s *Server) addOperationPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.repo.GetOperation(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		writeError(w, 400, "照片读取失败，限 4 MB")
		return
	}
	size, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || size.Width > 2048 || size.Height > 2048 {
		writeError(w, 400, "需要长边不超过 2048 像素的 JPEG 照片")
		return
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		writeError(w, 400, "照片文件损坏")
		return
	}
	digest, err := s.repo.AddOperationPhoto(r.Context(), id, data)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 201, digest)
}
func (s *Server) deleteOperationPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.repo.DeleteOperationPhoto(r.Context(), id, chi.URLParam(r, "digest")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(204)
}
