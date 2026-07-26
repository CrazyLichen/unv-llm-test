package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/config"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// MaterialLibraryService 素材库业务逻辑层
type MaterialLibraryService struct {
	repo *repository.MaterialLibraryRepo
	// uploadDir 上传文件存储根目录
	uploadDir string
	// maxImageSize 单个图片文件最大大小
	maxImageSize int64
	// maxImageCount 单次最多上传图片数量
	maxImageCount int
	// maxImageBatchSize 单次上传请求总体积最大值
	maxImageBatchSize int64
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewMaterialLibraryService 创建素材库服务实例
func NewMaterialLibraryService(repo *repository.MaterialLibraryRepo, uploadCfg *config.UploadConfig) *MaterialLibraryService {
	return &MaterialLibraryService{
		repo:              repo,
		uploadDir:         uploadCfg.Dir,
		maxImageSize:      uploadCfg.MaxImageSize,
		maxImageCount:     uploadCfg.MaxImageCount,
		maxImageBatchSize: uploadCfg.MaxImageBatchSize,
	}
}

// Create 创建素材库
func (s *MaterialLibraryService) Create(ctx context.Context, req *model.CreateMaterialLibraryReq) (*model.MaterialLibrary, error) {
	id, err := generateLibraryID()
	if err != nil {
		return nil, fmt.Errorf("生成ID失败: %w", err)
	}

	now := common.NowFormatted()
	ml := &model.MaterialLibrary{
		Id:          id,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		FileCount:   0,
		TotalSize:   0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, ml); err != nil {
		return nil, err
	}

	slog.Info("创建素材库成功", "id", id, "name", req.Name, "type", req.Type)
	return ml, nil
}

// GetByID 按 ID 查询素材库
func (s *MaterialLibraryService) GetByID(ctx context.Context, id string) (*model.MaterialLibrary, error) {
	ml, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ml == nil {
		return nil, common.ErrMaterialLibNotFound
	}
	return ml, nil
}

// List 分页查询素材库列表
func (s *MaterialLibraryService) List(ctx context.Context, page, pageSize int, libType string) ([]model.MaterialLibrary, int, error) {
	return s.repo.List(ctx, page, pageSize, libType)
}

// Update 更新素材库
func (s *MaterialLibraryService) Update(ctx context.Context, id string, req *model.UpdateMaterialLibraryReq) error {
	ml, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ml == nil {
		return common.ErrMaterialLibNotFound
	}
	if err := s.repo.Update(ctx, id, req); err != nil {
		return err
	}
	slog.Info("更新素材库成功", "id", id)
	return nil
}

// Delete 删除素材库（级联删除文件和磁盘文件）
func (s *MaterialLibraryService) Delete(ctx context.Context, id string) error {
	ml, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ml == nil {
		return common.ErrMaterialLibNotFound
	}

	// 检查是否有关联任务
	hasRelated, err := s.repo.HasRelatedTasks(ctx, id)
	if err != nil {
		return err
	}
	if hasRelated {
		return common.ErrMaterialLibBound
	}

	// 查询所有文件记录
	files, _, err := s.repo.ListFiles(ctx, id, 1, 10000, "")
	if err != nil {
		return err
	}

	// 删除磁盘文件
	for _, f := range files {
		fullPath := filepath.Join(s.uploadDir, f.StoragePath)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("删除磁盘文件失败", "path", fullPath, "error", err)
		}
		// 删除分片临时目录
		if f.UploadStatus == "Uploading" || f.UploadStatus == "Merging" {
			chunkDir := filepath.Join(s.uploadDir, filepath.Dir(f.StoragePath), "chunks")
			os.RemoveAll(chunkDir)
		}
	}

	// 删除素材库目录
	libDir := filepath.Join(s.uploadDir, strings.ToLower(ml.Type)+"s", id)
	os.RemoveAll(libDir)

	// 清理空类型目录（images/ 或 videos/）
	s.removeEmptyParentDirs(filepath.Dir(libDir))

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	slog.Info("删除素材库成功", "id", id, "deletedFiles", len(files))
	return nil
}

// UploadImages 批量上传图片
func (s *MaterialLibraryService) UploadImages(ctx context.Context, libraryId string, form *gin.Context) (*model.UploadImageResp, error) {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return nil, err
	}
	if ml == nil {
		return nil, common.ErrMaterialLibNotFound
	}
	if ml.Type != "Image" {
		return nil, common.ErrLibTypeMismatch
	}

	form.Request.Body = http.MaxBytesReader(form.Writer, form.Request.Body, s.maxImageBatchSize)

	multipartForm, err := form.MultipartForm()
	if err != nil {
		return nil, common.NewErrParamValidation("解析上传表单失败: " + err.Error())
	}

	files := multipartForm.File["files"]
	if len(files) == 0 {
		return nil, common.NewErrParamValidation("未选择图片文件")
	}
	if len(files) > s.maxImageCount {
		return nil, common.NewErrParamValidation(fmt.Sprintf("单次最多上传 %d 张图片", s.maxImageCount))
	}

	// 确保目录存在
	imgDir := filepath.Join(s.uploadDir, "images", libraryId)
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return nil, common.NewErrFileUploadFailed("创建存储目录失败")
	}

	var uploaded []model.MaterialFile
	for _, fh := range files {
		if fh.Size > s.maxImageSize {
			slog.Warn("图片文件超过大小限制", "fileName", fh.Filename, "size", fh.Size)
			continue
		}

		fileId, err := generateFileID()
		if err != nil {
			slog.Error("生成文件ID失败", "error", err)
			continue
		}

		ext := filepath.Ext(fh.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		storagePath := filepath.Join("images", libraryId, fileId+ext)
		accessUrl := filepath.Join("/uploads", storagePath)
		fullPath := filepath.Join(s.uploadDir, storagePath)

		src, err := fh.Open()
		if err != nil {
			slog.Error("打开上传文件失败", "fileName", fh.Filename, "error", err)
			continue
		}

		dst, err := os.Create(fullPath)
		if err != nil {
			src.Close()
			slog.Error("创建目标文件失败", "path", fullPath, "error", err)
			continue
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			os.Remove(fullPath)
			slog.Error("写入文件失败", "fileName", fh.Filename, "error", err)
			continue
		}

		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		mf := &model.MaterialFile{
			Id:           fileId,
			LibraryId:    libraryId,
			FileName:     fh.Filename,
			StoragePath:  filepath.ToSlash(storagePath),
			AccessUrl:    filepath.ToSlash(accessUrl),
			FileSize:     written,
			MimeType:     mimeType,
			UploadStatus: "Completed",
			CreatedAt:    common.NowFormatted(),
		}

		if err := s.repo.CreateFile(ctx, mf); err != nil {
			os.Remove(fullPath)
			continue
		}

		uploaded = append(uploaded, *mf)
	}

	// 更新素材库统计
	if err := s.repo.UpdateLibraryStats(ctx, libraryId); err != nil {
		slog.Error("更新素材库统计失败", "libraryId", libraryId, "error", err)
	}

	slog.Info("批量上传图片完成", "libraryId", libraryId, "uploadedCount", len(uploaded))
	return &model.UploadImageResp{
		UploadedCount: len(uploaded),
		Files:         uploaded,
	}, nil
}

// InitVideoUpload 初始化视频上传
func (s *MaterialLibraryService) InitVideoUpload(ctx context.Context, libraryId string, req *model.InitVideoUploadReq) (*model.InitVideoUploadResp, error) {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return nil, err
	}
	if ml == nil {
		return nil, common.ErrMaterialLibNotFound
	}
	if ml.Type != "Video" {
		return nil, common.ErrLibTypeMismatch
	}

	// 检查同名已完成或合并中的文件
	existing, err := s.repo.FindCompletedOrMergingFileByName(ctx, libraryId, req.FileName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, common.NewErrParamValidation("同名视频文件已存在")
	}

	// 断点续传：检查同名上传中的文件
	existingUploading, err := s.repo.FindUploadingFileByName(ctx, libraryId, req.FileName)
	if err != nil {
		return nil, err
	}
	if existingUploading != nil && existingUploading.UploadId != nil {
		slog.Info("断点续传：返回已有上传标识", "uploadId", *existingUploading.UploadId, "fileName", req.FileName)
		chunkCount := int32(0)
		if existingUploading.TotalChunks != nil {
			chunkCount = *existingUploading.TotalChunks
		}
		return &model.InitVideoUploadResp{
			UploadId:   *existingUploading.UploadId,
			ChunkCount: chunkCount,
		}, nil
	}

	// 创建新的上传记录
	fileId, err := generateFileID()
	if err != nil {
		return nil, fmt.Errorf("生成文件ID失败: %w", err)
	}

	uploadIdBytes := make([]byte, 16)
	if _, err := rand.Read(uploadIdBytes); err != nil {
		return nil, fmt.Errorf("生成UploadId失败: %w", err)
	}
	uploadId := "upload_" + hex.EncodeToString(uploadIdBytes)

	chunkCount := int32(req.FileSize / int64(req.ChunkSize))
	if req.FileSize%int64(req.ChunkSize) != 0 {
		chunkCount++
	}

	ext := filepath.Ext(req.FileName)
	if ext == "" {
		ext = ".mp4"
	}
	storagePath := filepath.Join("videos", libraryId, fileId+ext)
	accessUrl := filepath.Join("/uploads", storagePath)
	uploadedChunks := int32(0)

	mf := &model.MaterialFile{
		Id:             fileId,
		LibraryId:      libraryId,
		FileName:       req.FileName,
		StoragePath:    filepath.ToSlash(storagePath),
		AccessUrl:      filepath.ToSlash(accessUrl),
		FileSize:       req.FileSize,
		MimeType:       "video/mp4",
		UploadStatus:   "Uploading",
		TotalChunks:    &chunkCount,
		UploadedChunks: &uploadedChunks,
		UploadId:       &uploadId,
		CreatedAt:      common.NowFormatted(),
	}

	if err := s.repo.CreateFile(ctx, mf); err != nil {
		return nil, err
	}

	// 创建分片临时目录
	chunkDir := filepath.Join(s.uploadDir, "videos", libraryId, "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return nil, common.NewErrFileUploadFailed("创建分片目录失败")
	}

	slog.Info("初始化视频上传", "libraryId", libraryId, "uploadId", uploadId, "chunkCount", chunkCount)
	return &model.InitVideoUploadResp{
		UploadId:   uploadId,
		ChunkCount: chunkCount,
	}, nil
}

// UploadChunk 上传视频分片
func (s *MaterialLibraryService) UploadChunk(ctx context.Context, uploadId string, chunkIndex int32, fileHeader *multipart.FileHeader) error {
	mf, err := s.repo.GetFileByUploadId(ctx, uploadId)
	if err != nil {
		return err
	}
	if mf == nil {
		return common.NewErrParamValidation("无效的上传标识")
	}
	if mf.UploadStatus != "Uploading" {
		return common.NewErrParamValidation("文件不在上传中状态")
	}

	// 写入分片临时文件
	chunkDir := filepath.Join(s.uploadDir, filepath.Dir(mf.StoragePath), "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return common.NewErrFileUploadFailed("创建分片目录失败")
	}

	chunkPath := filepath.Join(chunkDir, mf.Id+".part."+strconv.Itoa(int(chunkIndex)))
	src, err := fileHeader.Open()
	if err != nil {
		return common.NewErrFileUploadFailed("打开分片数据失败")
	}
	defer src.Close()

	dst, err := os.Create(chunkPath)
	if err != nil {
		return common.NewErrFileUploadFailed("创建分片文件失败")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(chunkPath)
		return common.NewErrFileUploadFailed("写入分片数据失败")
	}

	// 更新已上传分片数
	currentUploaded := int32(0)
	if mf.UploadedChunks != nil {
		currentUploaded = *mf.UploadedChunks
	}
	newUploaded := currentUploaded + 1
	s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
		"uploaded_chunks": newUploaded,
	})

	slog.Info("上传分片成功", "uploadId", uploadId, "chunkIndex", chunkIndex, "uploaded", newUploaded)
	return nil
}

// CompleteVideoUpload 完成视频上传（异步合并）
func (s *MaterialLibraryService) CompleteVideoUpload(ctx context.Context, uploadId string) error {
	mf, err := s.repo.GetFileByUploadId(ctx, uploadId)
	if err != nil {
		return err
	}
	if mf == nil {
		return common.NewErrParamValidation("无效的上传标识")
	}
	if mf.UploadStatus != "Uploading" {
		return common.NewErrParamValidation("文件不在上传中状态")
	}

	// 校验所有分片是否已上传完毕
	if mf.TotalChunks != nil && mf.UploadedChunks != nil && *mf.UploadedChunks < *mf.TotalChunks {
		return common.NewErrChunkUploadFailed(fmt.Sprintf("分片未全部上传完毕 (%d/%d)", *mf.UploadedChunks, *mf.TotalChunks))
	}

	// 更新状态为 Merging
	totalChunks := *mf.TotalChunks
	s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
		"upload_status":   "Merging",
		"uploaded_chunks": totalChunks,
	})

	// 异步合并
	go s.mergeChunksAsync(mf)

	slog.Info("视频上传完成，开始异步合并", "uploadId", uploadId, "fileId", mf.Id)
	return nil
}

// ListFiles 查询素材文件列表
func (s *MaterialLibraryService) ListFiles(ctx context.Context, libraryId string, page, pageSize int, uploadStatus string) ([]model.MaterialFileProgress, int, error) {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return nil, 0, err
	}
	if ml == nil {
		return nil, 0, common.ErrMaterialLibNotFound
	}

	files, total, err := s.repo.ListFiles(ctx, libraryId, page, pageSize, uploadStatus)
	if err != nil {
		return nil, 0, err
	}

	result := make([]model.MaterialFileProgress, 0, len(files))
	for _, f := range files {
		result = append(result, toFileProgress(&f))
	}

	return result, total, nil
}

// DeleteFile 删除素材文件
func (s *MaterialLibraryService) DeleteFile(ctx context.Context, libraryId string, fileId string) error {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return err
	}
	if ml == nil {
		return common.ErrMaterialLibNotFound
	}

	// 检查素材库是否已关联任务
	hasRelated, err := s.repo.HasRelatedTasks(ctx, libraryId)
	if err != nil {
		return err
	}
	if hasRelated {
		return common.ErrMaterialLibBound
	}

	mf, err := s.repo.GetFileByID(ctx, fileId)
	if err != nil {
		return err
	}
	if mf == nil || mf.LibraryId != libraryId {
		return common.ErrMaterialFileNotFound
	}

	// 删除磁盘文件
	fullPath := filepath.Join(s.uploadDir, mf.StoragePath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("删除磁盘文件失败", "path", fullPath, "error", err)
	}

	// 清理空父目录（素材库目录、类型目录）
	s.removeEmptyParentDirs(filepath.Dir(fullPath))

	if err := s.repo.DeleteFile(ctx, fileId); err != nil {
		return err
	}

	// 更新素材库统计
	if err := s.repo.UpdateLibraryStats(ctx, libraryId); err != nil {
		slog.Error("更新素材库统计失败", "libraryId", libraryId, "error", err)
	}

	slog.Info("删除素材文件成功", "libraryId", libraryId, "fileId", fileId)
	return nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// generateLibraryID 生成 ml_{hex32} 格式的素材库 ID
func generateLibraryID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ml_" + hex.EncodeToString(bytes), nil
}

// generateFileID 生成 mf_{hex32} 格式的文件 ID
func generateFileID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "mf_" + hex.EncodeToString(bytes), nil
}

// toFileProgress 将 MaterialFile 转换为带进度的响应视图
func toFileProgress(mf *model.MaterialFile) model.MaterialFileProgress {
	progress := 1.0
	if mf.UploadStatus == "Uploading" && mf.TotalChunks != nil && *mf.TotalChunks > 0 {
		uploaded := int32(0)
		if mf.UploadedChunks != nil {
			uploaded = *mf.UploadedChunks
		}
		progress = float64(uploaded) / float64(*mf.TotalChunks)
	}
	return model.MaterialFileProgress{
		Id:             mf.Id,
		LibraryId:      mf.LibraryId,
		FileName:       mf.FileName,
		StoragePath:    mf.StoragePath,
		AccessUrl:      mf.AccessUrl,
		FileSize:       mf.FileSize,
		MimeType:       mf.MimeType,
		UploadStatus:   mf.UploadStatus,
		FailReason:     mf.FailReason,
		Progress:       progress,
		TotalChunks:    mf.TotalChunks,
		UploadedChunks: mf.UploadedChunks,
		CreatedAt:      mf.CreatedAt,
	}
}

// mergeChunksAsync 异步合并分片
func (s *MaterialLibraryService) mergeChunksAsync(mf *model.MaterialFile) {
	ctx := context.Background()
	chunkDir := filepath.Join(s.uploadDir, filepath.Dir(mf.StoragePath), "chunks")
	targetPath := filepath.Join(s.uploadDir, mf.StoragePath)

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		failReason := "创建目标目录失败: " + err.Error()
		s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
			"upload_status": "Failed",
			"fail_reason":   failReason,
		})
		slog.Error("视频合并失败", "fileId", mf.Id, "error", err)
		return
	}

	// 创建目标文件
	dst, err := os.Create(targetPath)
	if err != nil {
		failReason := "创建目标文件失败: " + err.Error()
		s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
			"upload_status": "Failed",
			"fail_reason":   failReason,
		})
		slog.Error("视频合并失败", "fileId", mf.Id, "error", err)
		return
	}
	defer dst.Close()

	// 按序合并分片
	totalChunks := int32(0)
	if mf.TotalChunks != nil {
		totalChunks = *mf.TotalChunks
	}

	for i := int32(0); i < totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, mf.Id+".part."+strconv.Itoa(int(i)))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			dst.Close()
			os.Remove(targetPath)
			failReason := fmt.Sprintf("打开分片 %d 失败: %s", i, err.Error())
			s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
				"upload_status": "Failed",
				"fail_reason":   failReason,
			})
			slog.Error("视频合并失败", "fileId", mf.Id, "chunkIndex", i, "error", err)
			return
		}
		if _, err := io.Copy(dst, chunkFile); err != nil {
			chunkFile.Close()
			dst.Close()
			os.Remove(targetPath)
			failReason := fmt.Sprintf("写入分片 %d 失败: %s", i, err.Error())
			s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
				"upload_status": "Failed",
				"fail_reason":   failReason,
			})
			slog.Error("视频合并失败", "fileId", mf.Id, "chunkIndex", i, "error", err)
			return
		}
		chunkFile.Close()
	}
	dst.Close()

	// 合并成功，清理分片临时文件
	os.RemoveAll(chunkDir)

	// 更新文件状态
	s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
		"upload_status":   "Completed",
		"total_chunks":    nil,
		"uploaded_chunks": nil,
		"upload_id":       nil,
	})

	// 更新素材库统计
	s.repo.UpdateLibraryStats(ctx, mf.LibraryId)

	slog.Info("视频合并成功", "fileId", mf.Id, "libraryId", mf.LibraryId)
}

// removeEmptyParentDirs 从 dir 开始向上逐级删除空目录，直到 uploadDir 为止
func (s *MaterialLibraryService) removeEmptyParentDirs(dir string) {
	for {
		if dir == s.uploadDir || dir == "" || dir == "." {
			break
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			slog.Warn("清理空目录失败", "dir", dir, "error", err)
			break
		}
		slog.Info("清理空目录", "dir", dir)
		dir = filepath.Dir(dir)
	}
}
