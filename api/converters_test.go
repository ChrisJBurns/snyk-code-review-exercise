package api_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/snyk/snyk-code-review-exercise/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStructure struct {
	FirstName string
	LastName  string
}

func TestConversionFromJsonToDataStructure(t *testing.T) {
	var converter api.Converter

	stringBody := io.NopCloser(strings.NewReader("{\"firstName\":\"Chris\",\"lastName\":\"Burns\"}"))
	var testStructure TestStructure
	err := converter.Unmarshall(stringBody, &testStructure)
	require.Nil(t, err)

	expected := &TestStructure{
		FirstName: "Chris",
		LastName:  "Burns",
	}
	assert.Equal(t, &testStructure, expected)
}

func TestConversionFromJsonToDataStructureWithErrorReadingResponseBody(t *testing.T) {
	converter := &api.Converter{
		Read: &StubRead{},
	}

	err := converter.Unmarshall(converter.Read, nil)
	assert.NotNil(t, err)
	assert.Equal(t, "Error: error reading response body", err.Error())
}

type StubRead struct {
}

func (s *StubRead) Read(p []byte) (n int, err error) {
	return 0, errors.New("error reading response body")
}
