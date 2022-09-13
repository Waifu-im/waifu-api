import typing
from enum import Enum

from pydantic import constr, BaseModel
from pydantic import constr, BaseModel, ValidationError, BoolError, validator

DEFAULT_REGEX = constr(regex="^[A-Za-z0-9_.-]+$")


class CustomBool(str, Enum):
    true = 'true'
    false = 'false'
    null = 'null'

    @classmethod
    def _missing_(cls, name):
        for member in cls:
            if member.name.lower() == name.lower():
                return member


class OrderByType(str, Enum):
    favourite = "FAVOURITES"
    uploaded_at = "UPLOADED_AT"


class FavOrderByType(str, Enum):
    favourite = "FAVOURITES"
    uploaded_at = "UPLOADED_AT"
    liked_at = "LIKED_AT"


class ImageOrientation(str, Enum):
    landscape = "LANDSCAPE"
    portrait = "PORTRAIT"


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
        self.url = "https://cdn.waifu.im/" + str(self.image_id) + self.extension
        self.preview_url = "https://waifu.im/preview/" + str(self.image_id) + '/'

    def __hash__(self):
        return int(self.image_id)

    def __eq__(self, o):
        return o == self.__hash__()


class PartialImage:
    def __init__(self, image_id, extension):
        self.image_id = image_id
        self.extension = extension
        self.filename = str(self.image_id) + self.extension


class Tag:
    def __init__(self, tag_id, name, description, is_nsfw):
        self.tag_id = int(tag_id)
        self.name = name
        self.description = description
        self.is_nsfw = is_nsfw

    def __hash__(self):
        return self.tag_id

    def __eq__(self, o):
        return o == self.__hash__()


class TagModel(BaseModel):
    tag_id: int
    name: str
    description: str
    is_nsfw: bool = False


class ImageModel(BaseModel):
    signature: typing.Union[str]
    extension: str
    image_id: int
    favourites: int
    dominant_color: str
    source: typing.Union[str, None]
    uploaded_at: str
    is_nsfw: bool = False
    width: int
    height: int
    url: str
    preview_url: str
    tags: typing.List[TagModel]


class ImageResponseModel(BaseModel):
    images: typing.List[ImageModel]
