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
var ModelRequestRateLimitMutex sync.RWMutex

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
