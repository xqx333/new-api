package setting

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveUserModelRateLimits(t *testing.T) {
	t.Helper()

	original := ModelRequestRateLimitUserModel2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateModelRequestRateLimitUserModelByJSONString(original))
	})
}

func TestUpdateModelRequestRateLimitUserModelBuildsExactLookup(t *testing.T) {
	preserveUserModelRateLimits(t)

	raw := `[
		{"user_id":123,"model":"gpt-5.4","max_requests":20,"max_success":10},
		{"user_id":123,"model":"GPT-5.4","max_requests":5,"max_success":0},
		{"user_id":456,"model":"gemini-2.5-pro","max_requests":0,"max_success":8}
	]`
	require.NoError(t, UpdateModelRequestRateLimitUserModelByJSONString(raw))

	assert.True(t, HasUserModelRateLimit(123))
	assert.True(t, HasUserModelRateLimit(456))
	assert.False(t, HasUserModelRateLimit(789))

	total, success, found := GetUserModelRateLimit(123, "gpt-5.4")
	assert.True(t, found)
	assert.Equal(t, 20, total)
	assert.Equal(t, 10, success)

	total, success, found = GetUserModelRateLimit(123, "GPT-5.4")
	assert.True(t, found)
	assert.Equal(t, 5, total)
	assert.Zero(t, success)

	_, _, found = GetUserModelRateLimit(123, "Gpt-5.4")
	assert.False(t, found)
	assert.JSONEq(t, raw, ModelRequestRateLimitUserModel2JSONString())
}

func TestCheckModelRequestRateLimitUserModelRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "not array", raw: `{}`},
		{name: "null", raw: `null`},
		{name: "invalid user", raw: `[{"user_id":0,"model":"gpt-5.4","max_requests":1,"max_success":0}]`},
		{name: "empty model", raw: `[{"user_id":1,"model":"","max_requests":1,"max_success":0}]`},
		{name: "model whitespace", raw: `[{"user_id":1,"model":" gpt-5.4","max_requests":1,"max_success":0}]`},
		{name: "negative total", raw: `[{"user_id":1,"model":"gpt-5.4","max_requests":-1,"max_success":0}]`},
		{name: "negative success", raw: `[{"user_id":1,"model":"gpt-5.4","max_requests":0,"max_success":-1}]`},
		{name: "both unlimited", raw: `[{"user_id":1,"model":"gpt-5.4","max_requests":0,"max_success":0}]`},
		{name: "total overflow", raw: fmt.Sprintf(`[{"user_id":1,"model":"gpt-5.4","max_requests":%d,"max_success":0}]`, int64(math.MaxInt32)+1)},
		{name: "success overflow", raw: fmt.Sprintf(`[{"user_id":1,"model":"gpt-5.4","max_requests":0,"max_success":%d}]`, int64(math.MaxInt32)+1)},
		{name: "duplicate", raw: `[{"user_id":1,"model":"gpt-5.4","max_requests":1,"max_success":0},{"user_id":1,"model":"gpt-5.4","max_requests":2,"max_success":0}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, CheckModelRequestRateLimitUserModel(test.raw))
		})
	}
}

func TestInvalidUserModelUpdatePreservesCurrentRules(t *testing.T) {
	preserveUserModelRateLimits(t)

	require.NoError(t, UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":123,"model":"gpt-5.4","max_requests":20,"max_success":10}]`,
	))
	require.Error(t, UpdateModelRequestRateLimitUserModelByJSONString(
		`[{"user_id":123,"model":"gpt-5.4","max_requests":0,"max_success":0}]`,
	))

	total, success, found := GetUserModelRateLimit(123, "gpt-5.4")
	assert.True(t, found)
	assert.Equal(t, 20, total)
	assert.Equal(t, 10, success)
}

func TestCheckModelRequestRateLimitGroupAllowsUnlimitedSuccess(t *testing.T) {
	assert.NoError(t, CheckModelRequestRateLimitGroup(`{"default":[100,0],"unlimited":[0,0]}`))
}
