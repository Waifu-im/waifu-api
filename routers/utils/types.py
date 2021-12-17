from enum import Enum


class ImageType(str, Enum):
    sfw = "sfw"
    nsfw = "nsfw"


class ImageQueue:
    """Add images to a queue and get maxsize most recent files returned to avoid displaying"""

    def __init__(self, redis, listname, maxsize):
        self._redis = redis
        self.maxsize = maxsize
        self.listname = listname

    async def put(self, item):
        async with self._redis.pipeline(transaction=True) as pipe:
            await (
                pipe.lpush(self.listname, *item if isinstance(item, list) else item)
                .ltrim(self.listname, 0, self.maxsize - 1)
                .execute()
            )

    async def get(self):
        return await self._redis.lrange(self.listname, 0, -1)
