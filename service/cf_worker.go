package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"one-api/common"
	"one-api/setting"
	"strings"
	"time"
)

type limitedReadCloser struct {
	reader   io.Reader     // 实际的 Reader（会使用 io.LimitReader 包装）
	closer   io.Closer     // 原始 resp.Body，用于在必要时 Close
	limit    int64         // 最大允许读取字节数
	readSize int64         // 已经读取的字节数
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if l.readSize >= l.limit {
		// 一旦检测到超过限制，我们就返回错误
		return 0, fmt.Errorf("exceeded maximum allowed size (%d bytes)", l.limit)
	}

	// 如果本次读取会超限，就截断到最大可读
	if int64(len(p))+l.readSize > l.limit {
		p = p[:l.limit-l.readSize]
	}

	n, err := l.reader.Read(p)
	l.readSize += int64(n)
	if err == io.EOF && l.readSize >= l.limit {
		return 0, fmt.Errorf("exceeded maximum allowed size (%d bytes)", l.limit)
	}
	return n, err
}

func (l *limitedReadCloser) Close() error {
	return l.closer.Close()
}

func newLimitedReadCloser(body io.ReadCloser, limit int64) io.ReadCloser {
	return &limitedReadCloser{
		reader: io.LimitReader(body, limit), // io.LimitReader 保证不会多读，但默认会返回 EOF
		closer: body,
		limit:  limit,
	}
}

func DoDownloadRequest(originUrl string) (resp *http.Response, err error) {
	maxImageSize := (common.MaxImageSize * 1024 * 1024) + 1
	requestTimeout := common.RequestTimeout

	client := &http.Client{}
	if requestTimeout > 0 {
		client.Timeout = time.Duration(requestTimeout) * time.Second
	}

	if setting.EnableWorker() {
		common.SysLog(fmt.Sprintf("downloading file from worker: %s", originUrl))
		if !strings.HasPrefix(originUrl, "https") {
			return nil, fmt.Errorf("only support https url")
		}
		workerUrl := setting.WorkerUrl
		if !strings.HasSuffix(workerUrl, "/") {
			workerUrl += "/"
		}
		data := []byte(`{"url":"` + originUrl + `","key":"` + setting.WorkerValidKey + `"}`)
		resp, err = client.Post(workerUrl, "application/json", bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
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

	if maxImageSize > 0 {
		resp.Body = newLimitedReadCloser(resp.Body, int64(maxImageSize))
	}
	
	return resp, nil
}
