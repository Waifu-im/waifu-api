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


class Image:
    def __init__(self, **kwargs):
        for key, value in kwargs.items():
            setattr(self, key, value)
        self.url = "https://cdn.waifu.im/" + self.file + self.extension
        self.preview_url = "https://waifu.im/preview/?image=" + self.file + self.extension

    def __hash__(self):
        return int(self.image_id)

    def __eq__(self, o):
        return o == self.__hash__()


class PartialImage:
    def __init__(self, file, extension):
        self.file = file
        self.extension = extension
        self.filename = self.file + self.extension


class Tags:
    def __init__(self, tag_id, name, is_nsfw, description):
        self.tag_id = int(tag_id)
        self.name = name
        self.is_nsfw = bool(is_nsfw)
        self.description = description

    def __hash__(self):
        return self.tag_id

    def __eq__(self, o):
        return o == self.__hash__()
