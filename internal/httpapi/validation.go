package httpapi

import (
	"net/http"
	"strings"
)

func method(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Method != expected {
		w.Header().Set("Allow", expected)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}
func pathID(path, prefix string) string {
	return strings.TrimPrefix(strings.TrimSuffix(path, "/"), prefix)
}
func requireID(w http.ResponseWriter, id string) bool {
	if strings.TrimSpace(id) == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return false
	}
	return true
}
func requestValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}
func setNoCache(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }
func setJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setNoCache(w)
}
func statusForError(message string) int {
	switch {
	case strings.Contains(message, "未找到"):
		return http.StatusNotFound
	case strings.Contains(message, "冲突"):
		return http.StatusConflict
	case strings.Contains(message, "不能为空"):
		return http.StatusBadRequest
	default:
		return http.StatusUnprocessableEntity
	}
}
