package setting

import (
	"encoding/json"
	"fmt"
	"one-api/common"
	"sync"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitModel = map[string][2]int{}
// 针对单个用户的限流配置，优先级高于分组与默认配置
var ModelRequestRateLimitUser = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

// 全局限流配置
var GlobalRequestRateLimitCount = 0        // 全局总请求数限制，0表示不限制
var GlobalModelRateLimitModel = map[string]int{} // 全局模型限流配置 {"model": maxCount}
var GlobalRateLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
    // 拷贝一份，缩短持锁时间
    snapshot := make(map[string][2]int, len(ModelRequestRateLimitGroup))
    for k, v := range ModelRequestRateLimitGroup {
        snapshot[k] = v
    }
    ModelRequestRateLimitMutex.RUnlock()

    jsonBytes, err := json.Marshal(snapshot)
    if err != nil {
        common.SysError("error marshalling model ratio: " + err.Error())
    }
    return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	// 先在局部变量里解析，避免半更新
    tmp := make(map[string][2]int)
    if err := json.Unmarshal([]byte(jsonStr), &tmp); err != nil {
        return err
    }
	
	// 成功后再一次性替换，使用写锁
    ModelRequestRateLimitMutex.Lock()
    ModelRequestRateLimitGroup = tmp
    ModelRequestRateLimitMutex.Unlock()
    return nil
}

// ModelRequestRateLimitUser2JSONString 将用户限流配置转换为 JSON 字符串
func ModelRequestRateLimitUser2JSONString() string {
    ModelRequestRateLimitMutex.RLock()
    // 拷贝一份，缩短持锁时间
    snapshot := make(map[string][2]int, len(ModelRequestRateLimitUser))
    for k, v := range ModelRequestRateLimitUser {
        snapshot[k] = v
    }
    ModelRequestRateLimitMutex.RUnlock()

    jsonBytes, err := json.Marshal(snapshot)
    if err != nil {
        common.SysError("error marshalling user rate limit: " + err.Error())
    }
    return string(jsonBytes)
}

// UpdateModelRequestRateLimitUserByJSONString 通过 JSON 字符串更新用户限流配置
func UpdateModelRequestRateLimitUserByJSONString(jsonStr string) error {
    // 先在局部变量里解析，避免半更新
    tmp := make(map[string][2]int)
    if err := json.Unmarshal([]byte(jsonStr), &tmp); err != nil {
        return err
    }
    // 成功后再一次性替换，使用写锁
    ModelRequestRateLimitMutex.Lock()
    ModelRequestRateLimitUser = tmp
    ModelRequestRateLimitMutex.Unlock()
    return nil
}

// GetUserRateLimit 获取指定用户的限流配置
func GetUserRateLimit(userId string) (totalCount, successCount int, found bool) {
    ModelRequestRateLimitMutex.RLock()
    defer ModelRequestRateLimitMutex.RUnlock()

    if ModelRequestRateLimitUser == nil {
        return 0, 0, false
    }

    limits, ok := ModelRequestRateLimitUser[userId]
    if !ok {
        return 0, 0, false
    }
    return limits[0], limits[1], true
}

// CheckModelRequestRateLimitUser 检查用户限流配置的有效性
func CheckModelRequestRateLimitUser(jsonStr string) error {
    check := make(map[string][2]int)
    if err := json.Unmarshal([]byte(jsonStr), &check); err != nil {
        return err
    }
    for uid, limits := range check {
        if limits[0] < 0 || limits[1] < 1 {
            return fmt.Errorf("user %s has invalid rate limit values: [%d, %d]", uid, limits[0], limits[1])
        }
    }
    return nil
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := json.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
	}

	return nil
}

// ModelRequestRateLimitModel2JSONString 将模型限流配置转换为 JSON 字符串
func ModelRequestRateLimitModel2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	// 拷贝一份，缩短持锁时间
	snapshot := make(map[string][2]int, len(ModelRequestRateLimitModel))
	for k, v := range ModelRequestRateLimitModel {
		snapshot[k] = v
	}
	ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(snapshot)
	if err != nil {
		common.SysError("error marshalling model rate limit: " + err.Error())
	}
	return string(jsonBytes)
}

// UpdateModelRequestRateLimitModelByJSONString 通过 JSON 字符串更新模型限流配置
func UpdateModelRequestRateLimitModelByJSONString(jsonStr string) error {
	// 先在局部变量里解析，避免半更新
	tmp := make(map[string][2]int)
	if err := json.Unmarshal([]byte(jsonStr), &tmp); err != nil {
		return err
	}

	// 成功后再一次性替换，使用写锁
	ModelRequestRateLimitMutex.Lock()
	ModelRequestRateLimitModel = tmp
	ModelRequestRateLimitMutex.Unlock()
	return nil
}

// GetModelRateLimit 获取指定模型的限流配置
func GetModelRateLimit(model string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitModel == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitModel[model]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

// CheckModelRequestRateLimitModel 检查模型限流配置的有效性
func CheckModelRequestRateLimitModel(jsonStr string) error {
	checkModelRequestRateLimitModel := make(map[string][2]int)
	err := json.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitModel)
	if err != nil {
		return err
	}
	for model, limits := range checkModelRequestRateLimitModel {
		if limits[0] < 0 {
			return fmt.Errorf("model %s has invalid rate limit value: [%d, %d], first value (total count) must be >= 0", model, limits[0], limits[1])
		}
		// 第二个参数预留，目前不使用，但要求 >= 0
		if limits[1] < 0 {
			return fmt.Errorf("model %s has invalid rate limit value: [%d, %d], second value must be >= 0", model, limits[0], limits[1])
		}
	}

	return nil
}

// GlobalModelRateLimitModel2JSONString 将全局模型限流配置转换为JSON字符串
func GlobalModelRateLimitModel2JSONString() string {
	GlobalRateLimitMutex.RLock()
	snapshot := make(map[string]int, len(GlobalModelRateLimitModel))
	for k, v := range GlobalModelRateLimitModel {
		snapshot[k] = v
	}
	GlobalRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(snapshot)
	if err != nil {
		common.SysError("error marshalling global model rate limit: " + err.Error())
	}
	return string(jsonBytes)
}

// UpdateGlobalModelRateLimitModelByJSONString 通过JSON字符串更新全局模型限流配置
func UpdateGlobalModelRateLimitModelByJSONString(jsonStr string) error {
	tmp := make(map[string]int)
	if err := json.Unmarshal([]byte(jsonStr), &tmp); err != nil {
		return err
	}

	GlobalRateLimitMutex.Lock()
	GlobalModelRateLimitModel = tmp
	GlobalRateLimitMutex.Unlock()

	return nil
}

// GetGlobalModelRateLimit 获取指定模型的全局限流配置
func GetGlobalModelRateLimit(model string) (int, bool) {
	GlobalRateLimitMutex.RLock()
	defer GlobalRateLimitMutex.RUnlock()

	maxCount, found := GlobalModelRateLimitModel[model]
	if !found {
		return 0, false
	}
	return maxCount, true
}

// CheckGlobalModelRateLimitModel 检查全局模型限流配置的有效性
func CheckGlobalModelRateLimitModel(jsonStr string) error {
	checkGlobalModelRateLimitModel := make(map[string]int)
	err := json.Unmarshal([]byte(jsonStr), &checkGlobalModelRateLimitModel)
	if err != nil {
		return err
	}
	for model, maxCount := range checkGlobalModelRateLimitModel {
		if maxCount < 0 {
			return fmt.Errorf("model %s has invalid rate limit value: %d, must be >= 0", model, maxCount)
		}
	}

	return nil
}
