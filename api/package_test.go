package api_test

import (
	"net/http"
	"testing"

	"github.com/snyk/snyk-code-review-exercise/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPackageMetaWhenPackageDoesNotExist(t *testing.T) {
	var pkg api.Package
	_, err := pkg.FetchMeta("sdsdsdsdsdsds")
	require.NotNil(t, err)

	if err, ok := err.(*api.ErrorResponse); ok {
		assert.Equal(t, http.StatusNotFound, err.StatusCode)
	}
}

func TestFetchPackageMetaWhenPackageDoesExist(t *testing.T) {
	var pkg api.Package
	resp, err := pkg.FetchMeta("react")
	require.Nil(t, err)
	require.NotNil(t, resp.Versions)
}

func TestFetchPackageWhenPackageDoesExist(t *testing.T) {
	var pkg api.Package
	resp, err := pkg.FetchPackage("react", "16.13.0")
	require.Nil(t, err)
	require.NotNil(t, resp)
}

func TestFetchPackageWhenPackageDoesNotExist(t *testing.T) {
	var pkg api.Package
	_, err := pkg.FetchPackage("sdsdsdsdsdsdsyjrjytrgf", "100.13.0")
	require.NotNil(t, err)

	if err, ok := err.(*api.ErrorResponse); ok {
		assert.Equal(t, http.StatusNotFound, err.StatusCode)
	}
}
