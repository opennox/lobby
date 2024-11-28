package lobby_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	lobby "github.com/opennox/lobby"
)

func TestGetIP(t *testing.T) {
	api := lobby.NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/address", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Result().StatusCode)
	require.Equal(t, `{"data":{"ip":"192.0.2.1"}}`+"\n", rec.Body.String())
}

func TestGetIPFailed(t *testing.T) {
	api := lobby.NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/address", nil)
	req.RemoteAddr = ""
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
}
