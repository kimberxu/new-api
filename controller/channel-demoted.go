/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/pkg/channel_slowstream"
	"github.com/gin-gonic/gin"
)

// GetDemotedChannels 返回当前降级中的渠道及降级模型/剩余时长。
// data 为 map[channelId][]DemotionInfo（空时为空 map）。
func GetDemotedChannels(c *gin.Context) {
	demoted := channelslowstream.ListDemoted()
	if demoted == nil {
		demoted = map[int][]channelslowstream.DemotionInfo{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    demoted,
	})
}
