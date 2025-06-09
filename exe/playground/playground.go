package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/lakelink/auth-companion/api"
)

func main() {
	j := api.NewApiEnsureTokenRequest{
		OidcUserId: "323283794821906439",
		TokenName:  "test",
		TokenGroup: "ultra",
	}

	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1323/newapi/ensure_token", bytes.NewBuffer(b))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode, string(body), err)
}
