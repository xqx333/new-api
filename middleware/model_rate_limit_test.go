package middleware

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelRedisRateLimitUsesUTCRegardlessOfLocalTimezone(t *testing.T) {
	redisServer, redisClient := useRateLimitMiniRedis(t)
	previousLocation := time.Local
	time.Local = time.FixedZone("test-utc-plus-eight", 8*60*60)
	t.Cleanup(func() { time.Local = previousLocation })

	ctx := context.Background()
	recordKey := "rateLimit:model-utc-record"
	recordRedisRequest(ctx, redisClient, recordKey, 2)
	recorded, err := redisClient.LIndex(ctx, recordKey, 0).Result()
	require.NoError(t, err)
	recordedAt, err := time.Parse(modelRateLimitTimeFormat, recorded)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC(), recordedAt, 2*time.Second)

	checkKey := "rateLimit:model-utc-check"
	withinWindow := time.Now().UTC().Add(-30 * time.Second).Format(modelRateLimitTimeFormat)
	_, err = redisServer.Push(checkKey, withinWindow, withinWindow)
	require.NoError(t, err)
	allowed, err := checkRedisRateLimit(ctx, redisClient, checkKey, 2, 60)
	require.NoError(t, err)
	assert.False(t, allowed, "an existing UTC timestamp inside the window must remain limited on a non-UTC host")
}

func preserveModelRequestRateLimitSettings(t *testing.T) {
	t.Helper()

	originalEnabled := setting.ModelRequestRateLimitEnabled
	originalDuration := setting.ModelRequestRateLimitDurationMinutes
	originalTotal := setting.ModelRequestRateLimitCount
	originalSuccess := setting.ModelRequestRateLimitSuccessCount
	originalGroups := setting.ModelRequestRateLimitGroup2JSONString()
	originalUserModels := setting.ModelRequestRateLimitUserModel2JSONString()
	originalRedisEnabled := common.RedisEnabled

	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitDurationMinutes = 1
	setting.ModelRequestRateLimitCount = 0
	setting.ModelRequestRateLimitSuccessCount = 0
	require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(`{}`))
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(`[]`))
	common.RedisEnabled = false

	t.Cleanup(func() {
		setting.ModelRequestRateLimitEnabled = originalEnabled
		setting.ModelRequestRateLimitDurationMinutes = originalDuration
		setting.ModelRequestRateLimitCount = originalTotal
		setting.ModelRequestRateLimitSuccessCount = originalSuccess
		require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(originalGroups))
		require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(originalUserModels))
		common.RedisEnabled = originalRedisEnabled
	})
}

func newModelRateLimitTestRouter(statusCode int, handler func(*gin.Context)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyStorageCleanup())
	router.Use(func(c *gin.Context) {
		userId, _ := strconv.Atoi(c.GetHeader("X-Test-User-Id"))
		c.Set("id", userId)
		if group := c.GetHeader("X-Test-Group"); group != "" {
			common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		}
		c.Next()
	})
	router.Use(ModelRequestRateLimit())
	router.Any("/*path", func(c *gin.Context) {
		if handler != nil {
			handler(c)
			return
		}
		c.Status(statusCode)
	})
	return router
}

func performModelRateLimitRequest(router http.Handler, method, target, contentType, body string, userId int, group string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Test-User-Id", strconv.Itoa(userId))
	if group != "" {
		request.Header.Set("X-Test-Group", group)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestUserModelRateLimitIsolatesUserAndModel(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":1001,"model":"gpt-5.4","max_requests":1,"max_success":0}]`,
	))
	router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 1001, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 1001, "").Code)
	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4-mini"}`, 1001, "").Code)
	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 1002, "").Code)
}

func TestUserModelRateLimitPrecedenceAndFallback(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	setting.ModelRequestRateLimitCount = 1
	require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(`{"vip":[2,0]}`))
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":2001,"model":"gpt-5.4","max_requests":3,"max_success":0}]`,
	))
	router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

	for range 3 {
		assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 2001, "vip").Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 2001, "vip").Code)

	for range 2 {
		assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-4.1"}`, 2001, "vip").Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-4.1"}`, 2001, "vip").Code)

	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-4.1"}`, 2002, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-4.1"}`, 2002, "").Code)
}

func TestMemorySuccessRateLimitRecordsOnlySuccessfulRequests(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":3001,"model":"gpt-5.4","max_requests":0,"max_success":1}]`,
	))

	responseStatus := http.StatusInternalServerError
	router := newModelRateLimitTestRouter(0, func(c *gin.Context) {
		c.Status(responseStatus)
	})

	assert.Equal(t, http.StatusInternalServerError, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 3001, "").Code)
	responseStatus = http.StatusNoContent
	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 3001, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 3001, "").Code)
}

func TestRedisUserModelSuccessRateLimit(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	redisServer, _ := useRateLimitMiniRedis(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":4001,"model":"gpt-5.4","max_requests":0,"max_success":1}]`,
	))
	router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 4001, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 4001, "").Code)
	assert.True(t, redisServer.Exists("rateLimit:MRRLS:4001:model:gpt-5.4"))
	assert.False(t, redisServer.Exists("rateLimit:MRRLS:4001"))
}

func TestRedisUserModelTotalRateLimit(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	redisServer, _ := useRateLimitMiniRedis(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":4002,"model":"gpt-5.4","max_requests":1,"max_success":0}]`,
	))
	router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 4002, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/chat/completions", "application/json", `{"model":"gpt-5.4"}`, 4002, "").Code)
	assert.True(t, redisServer.Exists("rateLimit:4002:model:gpt-5.4"))
	assert.False(t, redisServer.Exists("rateLimit:4002"))
}

type countingReadCloser struct {
	reader io.Reader
	reads  *atomic.Int32
}

func (r *countingReadCloser) Read(data []byte) (int, error) {
	r.reads.Add(1)
	return r.reader.Read(data)
}

func (r *countingReadCloser) Close() error {
	return nil
}

func TestModelRateLimitSkipsBodyWhenUserHasNoSpecificRules(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":5999,"model":"gpt-5.4","max_requests":1,"max_success":0}]`,
	))
	router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

	var reads atomic.Int32
	body := `{"model":"gpt-5.4","input":"body must remain unread"}`
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Body = &countingReadCloser{reader: strings.NewReader(body), reads: &reads}
	request.ContentLength = int64(len(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-User-Id", "5001")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Zero(t, reads.Load())
}

func TestModelRequestCachePreservesOriginalBodyAndCompactModel(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":6001,"model":"gpt-5.4","max_requests":1,"max_success":0}]`,
	))

	requestBody := `{"model":"gpt-5.4","input":"hello"}`
	router := newModelRateLimitTestRouter(0, func(c *gin.Context) {
		modelRequest, _, err := getModelRequest(c)
		require.NoError(t, err)
		assert.Equal(t, ratio_setting.WithCompactModelSuffix("gpt-5.4"), modelRequest.Model)
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		assert.JSONEq(t, requestBody, string(body))
		c.Status(http.StatusNoContent)
	})

	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/responses/compact", "application/json", requestBody, 6001, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/responses/compact", "application/json", requestBody, 6001, "").Code)
}

func TestUserModelRateLimitExtractsModelsFromSupportedRequestShapes(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		target      string
		contentType string
		body        string
		model       string
	}{
		{name: "Gemini path", method: http.MethodPost, target: "/v1beta/models/gemini-2.5-pro:generateContent", contentType: "application/json", body: `{}`, model: "gemini-2.5-pro"},
		{name: "Realtime query", method: http.MethodGet, target: "/v1/realtime?model=gpt-4o-realtime-preview", contentType: "application/json", body: `{}`, model: "gpt-4o-realtime-preview"},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preserveModelRequestRateLimitSettings(t)
			userId := 7001 + index
			rule := `[{"user_id":` + strconv.Itoa(userId) + `,"model":"` + test.model + `","max_requests":1,"max_success":0}]`
			require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(rule))
			router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

			assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, test.method, test.target, test.contentType, test.body, userId, "").Code)
			assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, test.method, test.target, test.contentType, test.body, userId, "").Code)
		})
	}
}

func TestUserModelRateLimitExtractsMultipartModel(t *testing.T) {
	preserveModelRequestRateLimitSettings(t)
	require.NoError(t, setting.UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":8001,"model":"whisper-1","max_requests":1,"max_success":0}]`,
	))
	router := newModelRateLimitTestRouter(http.StatusNoContent, nil)

	buildBody := func() (string, string) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "whisper-1"))
		require.NoError(t, writer.Close())
		return body.String(), writer.FormDataContentType()
	}
	body, contentType := buildBody()
	assert.Equal(t, http.StatusNoContent, performModelRateLimitRequest(router, http.MethodPost, "/v1/audio/transcriptions", contentType, body, 8001, "").Code)
	body, contentType = buildBody()
	assert.Equal(t, http.StatusTooManyRequests, performModelRateLimitRequest(router, http.MethodPost, "/v1/audio/transcriptions", contentType, body, 8001, "").Code)
}
