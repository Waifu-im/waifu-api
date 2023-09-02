package redis

import (
	"context"
	"strconv"
)

func (redis Redis) GetUserIdFromToken(token string) (int64, error) {
	ctx := context.Background()
	userIdString, err := redis.client.Get(ctx, redis.configuration.TempTokensKeyPrefix+":"+token).Result()
	if err != nil {
		return 0, err
	}
	userId, err := strconv.ParseInt(userIdString, 10, 64)
	if err != nil {
		return 0, err
	}
	return userId, err
}
