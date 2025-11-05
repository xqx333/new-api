package middleware

import (
	"context"
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/common/limiter"
	"one-api/constant"
	"one-api/setting"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"   // 用户总请求
	ModelRequestRateLimitSuccessCountMark = "MRRLS"  // 用户成功请求
	ModelRequestRateLimitModelCountMark   = "MRRLM"  // 用户模型总请求
	GlobalRequestRateLimitCountMark       = "GRRL"   // 全局总请求
	GlobalModelRequestRateLimitCountMark  = "GRRLM"  // 全局模型总请求
)

// 检查Redis中的请求限制
func checkRedisRateLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	// 如果maxCount为0，表示不限制
	if maxCount == 0 {
		return true, nil
	}

	// 获取当前计数
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// 如果未达到限制，允许请求
	if length < int64(maxCount) {
		return true, nil
	}

	// 检查时间窗口
	oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
	oldTime, err := time.Parse(timeFormat, oldTimeStr)
	if err != nil {
		return false, err
	}

	nowTimeStr := time.Now().Format(timeFormat)
	nowTime, err := time.Parse(timeFormat, nowTimeStr)
	if err != nil {
		return false, err
	}
	// 如果在时间窗口内已达到限制，拒绝请求
	subTime := nowTime.Sub(oldTime).Seconds()
	if int64(subTime) < duration {
		rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
		return false, nil
	}

	return true, nil
}

// 记录Redis请求
func recordRedisRequest(ctx context.Context, rdb *redis.Client, key string, maxCount int) {
	// 如果maxCount为0，不记录请求
	if maxCount == 0 {
		return
	}

	now := time.Now().Format(timeFormat)
	rdb.LPush(ctx, key, now)
	rdb.LTrim(ctx, key, 0, int64(maxCount-1))
	rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
}

// Redis限流处理器（支持模型限流和全局限流）
func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, model string, modelTotalCount, modelSuccessCount int, globalTotalCount, globalModelCount int) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		// 特殊处理 userId = 999
		// if userId == "999" {
		// 	totalMaxCount = 100
		// 	successMaxCount = 100
		// }

		ctx := context.Background()
		rdb := common.RDB

		// 0. 检查全局总请求数限制（所有用户共享）
		if globalTotalCount > 0 {
			globalTotalKey := fmt.Sprintf("rateLimit:%s:global", GlobalRequestRateLimitCountMark)
			tb := limiter.New(ctx, rdb)
			allowed, err := tb.Allow(
				ctx,
				globalTotalKey,
				limiter.WithCapacity(int64(globalTotalCount)*duration),
				limiter.WithRate(int64(globalTotalCount)),
				limiter.WithRequested(duration),
			)

			if err != nil {
				fmt.Println("检查全局总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}

			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, "当前负载已饱和，请稍后再试")
				return
			}
		}

		// 0.1 检查全局模型限流（所有用户共享）
		if model != "" && globalModelCount > 0 {
			globalModelKey := fmt.Sprintf("rateLimit:%s:global:%s", GlobalModelRequestRateLimitCountMark, model)
			tb := limiter.New(ctx, rdb)
			allowed, err := tb.Allow(
				ctx,
				globalModelKey,
				limiter.WithCapacity(int64(globalModelCount)*duration),
				limiter.WithRate(int64(globalModelCount)),
				limiter.WithRequested(duration),
			)

			if err != nil {
				fmt.Println("检查全局模型请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}

			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前模型 %s 负载已饱和，请稍后再试", model))
				return
			}
		}

		// 1. 检查用户成功请求数限制
		successKey := fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, userId)
		allowed, err := checkRedisRateLimit(ctx, rdb, successKey, successMaxCount, duration)
		if err != nil {
			fmt.Println("检查成功请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			// abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前分组上游负载已饱和，请稍后再试"))
			return
		}

		// 2. 检查用户总请求数限制（当totalMaxCount为0时会自动跳过，使用令牌桶限流器）
		if totalMaxCount > 0 {
			totalKey := fmt.Sprintf("rateLimit:%s", userId)
			// 初始化
			tb := limiter.New(ctx, rdb)
			allowed, err = tb.Allow(
				ctx,
				totalKey,
				limiter.WithCapacity(int64(totalMaxCount)*duration),
				limiter.WithRate(int64(totalMaxCount)),
				limiter.WithRequested(duration),
			)

			if err != nil {
				fmt.Println("检查总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}

			if !allowed {
				// abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前分组上游负载已饱和，请稍后再试"))
				return
			}
		}

		// 3. 检查用户模型限流（如果配置了）
		if model != "" && modelTotalCount > 0 {
			modelTotalKey := fmt.Sprintf("rateLimit:%s:%s:%s", ModelRequestRateLimitModelCountMark, userId, model)
			tb := limiter.New(ctx, rdb)
			allowed, err = tb.Allow(
				ctx,
				modelTotalKey,
				limiter.WithCapacity(int64(modelTotalCount)*duration),
				limiter.WithRate(int64(modelTotalCount)),
				limiter.WithRequested(duration),
			)

			if err != nil {
				fmt.Println("检查模型总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}

			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前模型 %s 负载已饱和，请稍后再试", model))
				return
			}
		}

		// 4. 处理请求
		c.Next()

		// 5. 如果请求成功，记录成功请求
		if c.Writer.Status() < 400 {
			recordRedisRequest(ctx, rdb, successKey, successMaxCount)
		}
	}
}

// 内存限流处理器（支持模型限流和全局限流）
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, model string, modelTotalCount, modelSuccessCount int, globalTotalCount, globalModelCount int) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		totalKey := ModelRequestRateLimitCountMark + userId
		successKey := ModelRequestRateLimitSuccessCountMark + userId

		// 0. 检查全局总请求数限制（所有用户共享）
		if globalTotalCount > 0 {
			globalTotalKey := GlobalRequestRateLimitCountMark + "global"
			if !inMemoryRateLimiter.Request(globalTotalKey, globalTotalCount, duration) {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, "当前负载已饱和，请稍后再试")
				return
			}
		}

		// 0.1 检查全局模型限流（所有用户共享）
		if model != "" && globalModelCount > 0 {
			globalModelKey := GlobalModelRequestRateLimitCountMark + "global:" + model
			if !inMemoryRateLimiter.Request(globalModelKey, globalModelCount, duration) {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前模型 %s 负载已饱和，请稍后再试", model))
				return
			}
		}

		// 1. 检查用户总请求数限制（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(totalKey, totalMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 2. 检查用户成功请求数限制
		// 使用一个临时key来检查限制，这样可以避免实际记录
		checkKey := successKey + "_check"
		if !inMemoryRateLimiter.Request(checkKey, successMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 3. 检查用户模型限流（如果配置了）
		if model != "" && modelTotalCount > 0 {
			modelTotalKey := ModelRequestRateLimitModelCountMark + userId + ":" + model
			if !inMemoryRateLimiter.Request(modelTotalKey, modelTotalCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
		}

		// 4. 处理请求
		c.Next()

		// 5. 如果请求成功，记录到实际的成功请求计数中
		if c.Writer.Status() < 400 {
			inMemoryRateLimiter.Request(successKey, successMaxCount, duration)
		}
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		// 计算限流参数
		duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := setting.ModelRequestRateLimitCount
		successMaxCount := setting.ModelRequestRateLimitSuccessCount

		// 获取分组
		// group := c.GetString(constant.ContextKeyUserGroup)
		group := c.GetString("token_group")
		if group == "" {
			// group = c.GetString("token_group")
			group = c.GetString(constant.ContextKeyUserGroup)
		}

		//获取分组的限流配置
		groupTotalCount, groupSuccessCount, found := setting.GetGroupRateLimit(group)
		if found {
			totalMaxCount = groupTotalCount
			successMaxCount = groupSuccessCount
		}

		// 用户级限流（优先级高于分组与默认）
		userIdStr := strconv.Itoa(c.GetInt("id"))
		if uTotal, uSuccess, uFound := setting.GetUserRateLimit(userIdStr); uFound {
			totalMaxCount = uTotal
			successMaxCount = uSuccess
		}

		// 获取模型信息和模型限流配置
		var model string
		var modelTotalCount, modelSuccessCount int
		
		// 检查是否有模型限流配置
		setting.ModelRequestRateLimitMutex.RLock()
		hasModelRateLimit := len(setting.ModelRequestRateLimitModel) > 0
		setting.ModelRequestRateLimitMutex.RUnlock()
		
		// 如果配置了模型限流，获取模型信息
		if hasModelRateLimit {
			modelRequest, _, err := GetModelRequest(c)
			if err == nil && modelRequest != nil && modelRequest.Model != "" {
				model = modelRequest.Model
				
				// 获取该模型的限流配置
				mTotalCount, mSuccessCount, modelFound := setting.GetModelRateLimit(model)
				if modelFound && mTotalCount >= 0 && mSuccessCount >= 1 {
					modelTotalCount = mTotalCount
					modelSuccessCount = mSuccessCount
				}
			}
		}

		// 获取全局限流配置
		globalTotalCount := setting.GlobalRequestRateLimitCount
		var globalModelCount int
		
		// 如果有模型信息，获取该模型的全局限流配置
		if model != "" {
			if count, found := setting.GetGlobalModelRateLimit(model); found {
				globalModelCount = count
			}
		}

		// 根据存储类型选择并执行限流处理器
		if common.RedisEnabled {
			redisRateLimitHandler(duration, totalMaxCount, successMaxCount, model, modelTotalCount, modelSuccessCount, globalTotalCount, globalModelCount)(c)
		} else {
			memoryRateLimitHandler(duration, totalMaxCount, successMaxCount, model, modelTotalCount, modelSuccessCount, globalTotalCount, globalModelCount)(c)
		}
	}
}
