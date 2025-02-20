package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"one-api/common"
	"one-api/setting"
	"strings"
	"time"
)

// WorkerRequest Worker请求的数据结构
type WorkerRequest struct {
	URL     string            `json:"url"`
	Key     string            `json:"key"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// DoWorkerRequest 通过Worker发送请求
func DoWorkerRequest(req *WorkerRequest) (*http.Response, error) {
	if !setting.EnableWorker() {
		return nil, fmt.Errorf("worker not enabled")
	}
	if !strings.HasPrefix(req.URL, "https") {
		return nil, fmt.Errorf("only support https url")
	}

	workerUrl := setting.WorkerUrl
	if !strings.HasSuffix(workerUrl, "/") {
		workerUrl += "/"
	}

	// 序列化worker请求数据
	workerPayload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal worker payload: %v", err)
	}

	return http.Post(workerUrl, "application/json", bytes.NewBuffer(workerPayload))
}

func DoDownloadRequest(originUrl string) (resp *http.Response, err error) {
	maxImageSize := int64(common.MaxImageSize * 1024 * 1024)
	requestTimeout := common.RequestTimeout

	client := &http.Client{}
	if requestTimeout > 0 {
		client.Timeout = time.Duration(requestTimeout) * time.Second
	}

	if setting.EnableWorker() {
		common.SysLog(fmt.Sprintf("downloading file from worker: %s", originUrl))
		req := &WorkerRequest{
			URL: originUrl,
			Key: setting.WorkerValidKey,
		}
		return DoWorkerRequest(req)
	} else {
		common.SysLog(fmt.Sprintf("downloading from origin: %s", originUrl))
		req, err := http.NewRequest(http.MethodGet, originUrl, nil)
		if err != nil {
			return nil, err
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
	}

	// 如果未设置最大图片大小，则直接返回响应
	if maxImageSize <= 0 {
		return resp, nil
	}
	
	// 使用io.LimitReader限制读取的字节数
	limitedReader := io.LimitReader(resp.Body, maxImageSize+1) // 读多一个字节以检测是否超出限制

	// 读取数据到缓冲区
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// 检查是否超过最大图片大小
	if int64(len(data)) > maxImageSize {
		resp.Body.Close()
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxImageSize)
	}

	// 将读取的数据重新封装到新的ReadCloser中
	resp.Body = io.NopCloser(bytes.NewBuffer(data))

	return resp, nil
}
