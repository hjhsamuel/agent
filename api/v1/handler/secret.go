package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/v1/schema"
)

// GenerateSecret 生成新的加密密钥。
// @Summary 生成加密密钥
// @Description 生成新版本的加密密钥并追加保存到 .env，响应仅返回密钥版本。
// @Tags 密钥
// @Produce json
// @Security CookieAuth
// @Success 200 {object} common.Response{data=schema.GenerateSecretRsp} "生成成功"
// @Failure 401 {object} common.Response "缺少认证 Cookie"
// @Failure 500 {object} common.Response "生成或保存密钥失败"
// @Router /v1/secret [post]
func (a *Api) GenerateSecret(c *gin.Context) {
	version, err := a.srv.GenerateSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, &common.Response{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("generate secret: %v", err),
		})
	} else {
		c.JSON(http.StatusOK, &common.Response{
			Code: http.StatusOK,
			Data: &schema.GenerateSecretRsp{
				Version: version,
			},
		})
	}
}
