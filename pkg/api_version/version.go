package api_version

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetVersionCommit(hostname string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://%s/api/version", hostname))
	if err != nil {
		return "", fmt.Errorf("http get failed: %v", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("error parsing returned json: %v", err)
	}

	if commitID, ok := data["git.commit.id"].(string); ok {
		return commitID, nil
	} else {
		return "", fmt.Errorf("commit ID not found or not a string")
	}
}
