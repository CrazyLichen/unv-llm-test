package controller

import (
	"github.com/gin-gonic/gin"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// SetupRouter 注册所有路由
func SetupRouter(r *gin.Engine, mcCtrl *ModelConfigController, mlCtrl *MaterialLibraryController, taskCtrl *TaskController, uploadDir string) {
	// 静态文件服务
	r.Static("/uploads", uploadDir)

	api := r.Group("/api")
	{
		mc := api.Group("/model-configs")
		{
			mc.POST("", mcCtrl.Create)
			mc.GET("", mcCtrl.List)
			mc.PUT("/:id", mcCtrl.Update)
			mc.DELETE("/:id", mcCtrl.Delete)
			mc.POST("/:id/test", mcCtrl.Test)
		}

		ml := api.Group("/material-libraries")
		{
			ml.POST("", mlCtrl.Create)
			ml.GET("", mlCtrl.List)
			ml.PUT("/:id", mlCtrl.Update)
			ml.DELETE("/:id", mlCtrl.Delete)
			ml.POST("/:id/images", mlCtrl.UploadImages)
			ml.POST("/:id/videos/init", mlCtrl.InitVideoUpload)
			ml.POST("/:id/videos/chunk", mlCtrl.UploadChunk)
			ml.POST("/:id/videos/complete", mlCtrl.CompleteVideoUpload)
			ml.GET("/:id/files", mlCtrl.ListFiles)
			ml.DELETE("/:id/files/:fileId", mlCtrl.DeleteFile)
		}

		tasks := api.Group("/tasks")
		{
			tasks.POST("", taskCtrl.Create)
			tasks.GET("", taskCtrl.List)
			tasks.DELETE("/:id", taskCtrl.Delete)
			tasks.PUT("/:id", taskCtrl.Update)
		}
	}
}
