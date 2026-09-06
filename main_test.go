package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetListenPortUsesAPIKey(t *testing.T) {
	const apiKey = "qbt_test_api_key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v2/app/preferences", r.URL.Path)
		assert.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"listen_port": 51413}`))
	}))
	defer server.Close()

	port, err := getListenPort(server.Client(), server.URL, apiKey)
	assert.NoError(t, err)
	assert.Equal(t, 51413, port)
}

func TestUpdateListenPortUsesAPIKey(t *testing.T) {
	const apiKey = "qbt_test_api_key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/app/setPreferences", r.URL.Path)
		assert.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, `{"listen_port": 51413}`, r.PostForm.Get("json"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	assert.NoError(t, updateListenPort(server.Client(), server.URL, apiKey, 51413))
}

func TestGetListenPortReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("bad api key"))
	}))
	defer server.Close()

	_, err := getListenPort(server.Client(), server.URL, "qbt_test_api_key")
	assert.Errorf(t, err, "getListenPort returned nil error, want status error")

	assert.Contains(t, err.Error(), "status code 403")
	assert.Contains(t, err.Error(), "bad api key")
}
