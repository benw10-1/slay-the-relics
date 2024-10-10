package api

import (
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
)

// TODO: stub for external clients, redis docker container in CI, tests for other endpoints

func TestDeckAPIHandler(t *testing.T) {
	router, err := New(nil, nil, nil)
	assert.NilError(t, err)

	testName := "testdeck" // TODO: UUID pkg for stuff like this

	// TODO: seed data with actual test data
	bigDeckStr := getBigDeckString()

	router.deckLists[testName] = &deck{buf: []byte(bigDeckStr)}

	stableMap, err := decompressDeck(bigDeckStr)
	assert.NilError(t, err)

	expectedOutput := getStableOutput(stableMap)

	handlerFn := router.Router.Handler()

	httpReq := httptest.NewRequest("GET", "/deck/"+testName, nil)

	w := httptest.NewRecorder()

	handlerFn.ServeHTTP(w, httpReq)

	assert.Equal(t, w.Code, 200, w.Body.String())

	assert.Equal(t, w.Body.String(), expectedOutput)
}
