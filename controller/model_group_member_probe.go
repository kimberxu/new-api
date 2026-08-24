package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// [personal] TestModelGroupItem probes one model-group member (a concrete
// channel + real upstream model) with a live chat request. A successful probe
// clears the model-level disable record of any source, mirroring TestChannel:
// this is the manual unban path for members auto-banned by processChannelError
// (or manually disabled) directly from the /model-groups page.
func TestModelGroupItem(c *gin.Context) {
	itemId, err := strconv.Atoi(c.Param("itemId"))
	if err != nil || itemId <= 0 {
		common.ApiError(c, fmt.Errorf("invalid item id"))
		return
	}
	item, err := model.GetModelGroupItem(itemId)
	if err != nil || item == nil {
		common.ApiError(c, fmt.Errorf("group item #%d not found", itemId))
		return
	}
	channel, err := model.GetChannelById(item.ChannelId, true)
	if err != nil {
		common.ApiError(c, fmt.Errorf("channel #%d not found", item.ChannelId))
		return
	}
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tik := time.Now()
	requestCtx := context.Background()
	if c.Request != nil {
		requestCtx = c.Request.Context()
	}
	result := testChannel(requestCtx, channel, testUserID, item.Model, "", shouldUseStreamForAutomaticChannelTest(channel))
	if result.localErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": result.localErr.Error(),
			"time":    0.0,
		})
		return
	}
	milliseconds := time.Since(tik).Milliseconds()
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if result.newAPIError != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"message":    result.newAPIError.Error(),
			"time":       consumedTime,
			"error_code": result.newAPIError.GetErrorCode(),
		})
		return
	}
	// Probe passed: clear any-source model-level disable (manual unban).
	if result.modelName != "" {
		_ = service.EnableChannelModel(item.ChannelId, result.modelName, "")
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
	})
}
