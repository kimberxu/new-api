package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

// [personal] registerModelGroupRoutes mounts the model-group management API.
// Groups are the first-class route table: group name = routable model name,
// members = real upstream models on concrete channels with member-level
// priority/weight overrides and a group-level param override.
func registerModelGroupRoutes(apiRouter *gin.RouterGroup) {
	modelGroupRoute := apiRouter.Group("/model-groups")
	modelGroupRoute.Use(middleware.AdminAuth())

	for _, route := range modelGroupPermissionRoutes {
		modelGroupRoute.Handle(route.method, route.path,
			middleware.RequirePermission(route.permission),
			route.handler,
		)
	}
}

var modelGroupPermissionRoutes = []permissionRoute{
	{method: http.MethodGet, path: "/", permission: authz.ChannelRead, handler: controller.ListModelGroups},
	{method: http.MethodPost, path: "/", permission: authz.ChannelWrite, handler: controller.CreateModelGroup},
	{method: http.MethodGet, path: "/:id", permission: authz.ChannelRead, handler: controller.GetModelGroup},
	{method: http.MethodPatch, path: "/:id", permission: authz.ChannelWrite, handler: controller.SetModelGroupEnabled},
	{method: http.MethodDelete, path: "/:id", permission: authz.ChannelWrite, handler: controller.DeleteModelGroup},
	{method: http.MethodPut, path: "/:id/param-override", permission: authz.ChannelWrite, handler: controller.SetModelGroupParamOverride},
	{method: http.MethodPost, path: "/rebuild", permission: authz.ChannelWrite, handler: controller.RebuildModelGroups},
	{method: http.MethodPost, path: "/:id/items", permission: authz.ChannelWrite, handler: controller.AddGroupItem},
	{method: http.MethodPatch, path: "/items/:itemId", permission: authz.ChannelWrite, handler: controller.UpdateGroupItem},
	{method: http.MethodDelete, path: "/items/:itemId", permission: authz.ChannelWrite, handler: controller.DeleteGroupItem},
	{method: http.MethodGet, path: "/:id/references", permission: authz.ChannelRead, handler: controller.ListGroupReferences},
	{method: http.MethodPost, path: "/:id/references", permission: authz.ChannelWrite, handler: controller.AddGroupReference},
	{method: http.MethodDelete, path: "/:id/references/:refGroupId", permission: authz.ChannelWrite, handler: controller.DeleteGroupReference},
}