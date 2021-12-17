from enum import Enum
class ImageType(str, Enum):
    sfw="sfw"
    nsfw="nsfw"

class ImageQueue:
    """Add images to a queue and get 30 most recent files returned to avoid displaying"""
    def __init__(self, maxsize):
        self._queue = []
        self.maxsize = maxsize

    def put(self, item):
        if isinstance(item, list):
            self._queue = item + self._queue[0:len(item)+1]
            del self._queue[self.maxsize+1:]
        else:
            self._queue.insert(0, item)
            del self._queue[self.maxsize+1:]

    def get(self):
        return self._queue

