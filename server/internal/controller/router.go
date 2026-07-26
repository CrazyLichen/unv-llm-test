package controller

import (
	"github.com/gin-gonic/gin"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// SetupRouter 注册所有路由
func SetupRouter(r *gin.Engine, mcCtrl *ModelConfigController) {
	api := r.Group("/api")
	{
		mc := api.Group("/model-configs")
		{
			mc.POST("", mcCtrl.Create)
			mc.GET("", mcCtrl.List)
			mc.PATCH("/:id", mcCtrl.Update)
			mc.DELETE("/:id", mcCtrl.Delete)
			mc.POST("/:id/test", mcCtrl.Test)
		}
	}
}
