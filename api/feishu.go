package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type FeishuUserInfoResponse struct {
	Code int `json:"code"`
	Data struct {
		AvatarBig       string `json:"avatar_big"`
		AvatarMiddle    string `json:"avatar_middle"`
		AvatarThumb     string `json:"avatar_thumb"`
		AvatarURL       string `json:"avatar_url"`
		Email           string `json:"email"`
		EmployeeNo      string `json:"employee_no"`
		EnName          string `json:"en_name"`
		EnterpriseEmail string `json:"enterprise_email"`
		Mobile          string `json:"mobile"`
		Name            string `json:"name"`
		OpenID          string `json:"open_id"`
		TenantKey       string `json:"tenant_key"`
		UnionID         string `json:"union_id"`
		UserID          string `json:"user_id"`
	} `json:"data"`
	Msg string `json:"msg"`
}

func SetupFeishuEndpoints(g *echo.Group) {
	g.GET("/user_info", handleFeishuUserInfo)
}

func handleFeishuUserInfo(c echo.Context) error {
	req, err := http.NewRequest("GET", "https://open.feishu.cn/open-apis/authen/v1/user_info", nil)
	if err != nil {
		return err
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, c.Request().Header.Get(echo.HeaderAuthorization))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var j FeishuUserInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&j)
	if err != nil {
		return err
	}

	if j.Code != 0 {
		return fmt.Errorf("%s, code=%d", j.Msg, j.Code)
	}

	return c.JSON(resp.StatusCode, j.Data)
}
