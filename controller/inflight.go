package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetInflightRequests returns a paginated list of currently in-flight relay
// requests (admin only). Each entry includes the request ID, selected channel,
// model name, request path, and start timestamp.
func GetInflightRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, err := service.List(pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, err := service.Count()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
