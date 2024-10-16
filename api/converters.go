package api

import (
	"encoding/json"
	"fmt"
	"io"
)

type Converter struct{}

func (c *Converter) Unmarshall(bodyResponse io.ReadCloser, parsedStructure interface{}) error {
	body, err := io.ReadAll(bodyResponse)
	if err != nil {
		return fmt.Errorf("Error reading response body: %v", err)
	}

	if err := json.Unmarshal([]byte(body), &parsedStructure); err != nil {
		return fmt.Errorf("Error parsing json response body into data structure: %v", err)
	}

	return nil
}
