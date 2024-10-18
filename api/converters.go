package api

import (
	"encoding/json"
	"fmt"
	"io"
)

type Converter struct {
	Read io.Reader
}

func (c *Converter) Unmarshall(bodyResponse io.Reader, parsedStructure interface{}) error {
	body, err := io.ReadAll(bodyResponse)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	if err := json.Unmarshal([]byte(body), &parsedStructure); err != nil {
		return fmt.Errorf("Error parsing json response body into data structure: %v", err)
	}

	return nil
}
