package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryRateLimiterCheckDoesNotRecordRequests(t *testing.T) {
	var rateLimiter InMemoryRateLimiter
	rateLimiter.Init(time.Minute)

	assert.True(t, rateLimiter.Check("success", 1, 60))
	assert.True(t, rateLimiter.Check("success", 1, 60))
	assert.True(t, rateLimiter.Request("success", 1, 60))
	assert.False(t, rateLimiter.Check("success", 1, 60))
}

func TestInMemoryRateLimiterTreatsZeroAsUnlimited(t *testing.T) {
	var rateLimiter InMemoryRateLimiter
	rateLimiter.Init(time.Minute)

	for range 3 {
		assert.True(t, rateLimiter.Check("success", 0, 60))
		assert.True(t, rateLimiter.Request("total", 0, 60))
	}
	assert.Empty(t, rateLimiter.store)
}
