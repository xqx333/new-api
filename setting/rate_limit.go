package setting

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitHideDetailsEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

type ModelRequestRateLimitUserModelRule struct {
	UserId      int    `json:"user_id"`
	Model       string `json:"model"`
	MaxRequests int    `json:"max_requests"`
	MaxSuccess  int    `json:"max_success"`
}

var modelRequestRateLimitUserModelRules = []ModelRequestRateLimitUserModelRule{}
var modelRequestRateLimitUserModelIndex = map[int]map[string]ModelRequestRateLimitUserModelRule{}
var modelRequestRateLimitUserModelMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := common.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	groupLimits := make(map[string][2]int)
	if err := common.UnmarshalJsonStr(jsonStr, &groupLimits); err != nil {
		return err
	}

	ModelRequestRateLimitMutex.Lock()
	ModelRequestRateLimitGroup = groupLimits
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
	err := common.UnmarshalJsonStr(jsonStr, &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 0 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
		if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
			return fmt.Errorf("group %s [%d, %d] has max rate limits value 2147483647", group, limits[0], limits[1])
		}
	}

	return nil
}

func ModelRequestRateLimitUserModel2JSONString() string {
	modelRequestRateLimitUserModelMutex.RLock()
	defer modelRequestRateLimitUserModelMutex.RUnlock()

	jsonBytes, err := common.Marshal(modelRequestRateLimitUserModelRules)
	if err != nil {
		common.SysLog("error marshalling user model rate limits: " + err.Error())
		return "[]"
	}
	return string(jsonBytes)
}

func parseModelRequestRateLimitUserModel(jsonStr string) ([]ModelRequestRateLimitUserModelRule, map[int]map[string]ModelRequestRateLimitUserModelRule, error) {
	var rules []ModelRequestRateLimitUserModelRule
	if err := common.UnmarshalJsonStr(jsonStr, &rules); err != nil {
		return nil, nil, err
	}
	if rules == nil {
		return nil, nil, fmt.Errorf("user model rate limits must be a JSON array")
	}

	index := make(map[int]map[string]ModelRequestRateLimitUserModelRule)
	for position, rule := range rules {
		if rule.UserId <= 0 {
			return nil, nil, fmt.Errorf("rule %d has invalid user_id %d", position+1, rule.UserId)
		}
		if rule.Model == "" || strings.TrimSpace(rule.Model) != rule.Model {
			return nil, nil, fmt.Errorf("rule %d has an empty model or surrounding whitespace", position+1)
		}
		if rule.MaxRequests < 0 || rule.MaxSuccess < 0 {
			return nil, nil, fmt.Errorf("rule %d has negative rate limit values", position+1)
		}
		if rule.MaxRequests > math.MaxInt32 || rule.MaxSuccess > math.MaxInt32 {
			return nil, nil, fmt.Errorf("rule %d has a rate limit value greater than 2147483647", position+1)
		}
		if rule.MaxRequests == 0 && rule.MaxSuccess == 0 {
			return nil, nil, fmt.Errorf("rule %d must limit max_requests or max_success", position+1)
		}

		modelRules, ok := index[rule.UserId]
		if !ok {
			modelRules = make(map[string]ModelRequestRateLimitUserModelRule)
			index[rule.UserId] = modelRules
		}
		if _, exists := modelRules[rule.Model]; exists {
			return nil, nil, fmt.Errorf("duplicate rate limit rule for user %d and model %s", rule.UserId, rule.Model)
		}
		modelRules[rule.Model] = rule
	}

	return rules, index, nil
}

func CheckModelRequestRateLimitUserModel(jsonStr string) error {
	_, _, err := parseModelRequestRateLimitUserModel(jsonStr)
	return err
}

func UpdateModelRequestRateLimitUserModelByJSONString(jsonStr string) error {
	rules, index, err := parseModelRequestRateLimitUserModel(jsonStr)
	if err != nil {
		return err
	}

	modelRequestRateLimitUserModelMutex.Lock()
	modelRequestRateLimitUserModelRules = rules
	modelRequestRateLimitUserModelIndex = index
	modelRequestRateLimitUserModelMutex.Unlock()
	return nil
}

func HasUserModelRateLimit(userId int) bool {
	modelRequestRateLimitUserModelMutex.RLock()
	defer modelRequestRateLimitUserModelMutex.RUnlock()

	return len(modelRequestRateLimitUserModelIndex[userId]) > 0
}

func GetUserModelRateLimit(userId int, model string) (totalCount, successCount int, found bool) {
	modelRequestRateLimitUserModelMutex.RLock()
	defer modelRequestRateLimitUserModelMutex.RUnlock()

	rule, found := modelRequestRateLimitUserModelIndex[userId][model]
	if !found {
		return 0, 0, false
	}
	return rule.MaxRequests, rule.MaxSuccess, true
}
