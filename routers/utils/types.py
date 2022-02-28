import typing
from enum import Enum

from pydantic import constr, BaseModel, ValidationError, BoolError, validator

BOOL_FALSE = {0, '0', 'off', 'f', 'false', 'n', 'no'}
BOOL_TRUE = {1, '1', 'on', 't', 'true', 'y', 'yes'}
# Yes it doesn't make sense but who cares?
BOOL_NONE = {'random', 'none', 'null'}


class BooleanNoneModel(BaseModel):
    is_nsfw: str = False

    @validator('is_nsfw')
    def bool_validator(cls, v) -> bool:
        if v is True or v is False or v is None:
            return v
        if isinstance(v, bytes):
            v = v.decode()
        if isinstance(v, str):
            v = v.lower()
        try:
            if v in BOOL_TRUE:
                return True
            if v in BOOL_FALSE:
                return False
            if v in BOOL_NONE:
                return None
        except TypeError:
            raise BoolError()
        raise BoolError()


DEFAULT_REGEX = constr(regex="^[A-Za-z0-9_.-]*$")


class OrderByType(str, Enum):
    favourite = "FAVOURITES"
    uploaded_at = "UPLOADED_AT"


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
    def __init__(self, tag_id, name, description, is_nsfw):
        self.tag_id = int(tag_id)
        self.name = name
        self.description = description
        self.is_nsfw = is_nsfw

    def __hash__(self):
        return self.tag_id

    def __eq__(self, o):
        return o == self.__hash__()
