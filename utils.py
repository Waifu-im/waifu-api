from werkzeug.datastructures import MultiDict
import aiohttp
import json
import os
import asyncpg


"""Endpoints infos"""
async def myendpoints(app,over18=None):
    async with app.pool.acquire() as conn:
        rt=await conn.fetch("SELECT * FROM Tags")
    
    if over18 is None:
        return {"sfw":[tag['name'] for tag in rt if not tag['is_nsfw'] ],"nsfw":[tag['name'] for tag in rt if tag['is_nsfw']],'example':'https://api.waifu.im/sfw/waifu/'}
    elif over18:
        return [tag['name'] for tag in rt if tag['is_nsfw']]
    else:
        return [tag['name'] for tag in rt if not tag['is_nsfw']]

async def myendpoints_info(app,over18=None):
    async with app.pool.acquire() as conn:
        rt=await conn.fetch("SELECT * FROM Tags")
    if over18 is None:
        return {"sfw":[to_dict(tag) for tag in rt if not tag['is_nsfw']],
                "nsfw":[to_dict(tag) for tag in rt if tag['is_nsfw']],'example':'https://api.waifu.im/sfw/waifu/'}
    elif over18:
        return [to_dict(tag) for tag in rt if tag['is_nsfw']]
    else:
        return [to_dict(tag) for tag in rt if not tag['is_nsfw']]


    



"""converters"""
def convert_bool(string):
    string=string.lower()
    try:
        string=json.loads(string)
    except Exception as e:
        print(e)
        raise ValueError
    return string

def format_to_image(string):
    if not string:
        raise ValueError
    images=[x for x in string.split(",")]
    try:
        return [Image(os.path.splitext(x)[0],os.path.splitext(x)[1]) for x in images]
    except:
        raise ValueError

def to_dict(obj):
    obj=dict(obj)
    obj['id']=int(obj['id'])
    return obj

"""Determine if an image is already or not in the User gallery for the toggle url param"""
async def wich_action(app,image,insert,delete,user_id,cursor):
    if not image:
        return
    async with app.pool.acquire() as conn:
        for im in image:
            rt=await cursor.fetchrow("SELECT image FROM FavImages WHERE user_id=$1 and image=$2",user_id,im.filename)
            if rt:
                delete.append(im)
            else:
                insert.append(im)

"""Utils to format into apropriate sql qury"""
def methodandimage(user_id,insert=None,delete=None):
    
    if insert:
        args=[(user_id,im.filename) for im in insert]
        return "INSERT INTO FavImages(user_id,image) VALUES($1,$2) ON CONFLICT (user_id,image) DO NOTHING",args
    elif delete:
        args=[(user_id,im.filename) for im in delete]
        return "DELETE FROM FavImages WHERE user_id=$1 and image=$2",args

def db_to_json(images):
    tagmapping=[]
    for im in images:
        im=dict(im)
        im['id']=int(im['id'])
        tagmapping.append((Tags(im.pop('id'),im.pop('name'),im.pop('is_nsfw'),im.pop('description')),im))
    tagmapping=MultiDict(tagmapping)
    default_tags=['ero','all']
    tags_=[]
    for tag,image in tagmapping.items():
        tag_images=tagmapping.getlist(tag.tag_id)
        tag_images=[dict(t,**{'url':'https://cdn.waifu.im/'+t['file']+t['extension']}) for t in tag_images]
        tags_.append(dict(vars(tag),**{'images':tag_images}))
    return tags_

"""Object Class"""
class Image:
    def __init__(self,filename,extension,dominant_color=None):
        self.filename=filename
        self.extension=extension
        self.fullfilename=filename+extension
        self.dominant_color=dominant_color

class Tags:
    def __init__(self,tag_id,name,is_nsfw,description):
        self.tag_id=tag_id
        self.name=name
        self.is_nsfw=bool(is_nsfw)
        self.description=description
    def __hash__(self):
        return self.tag_id
    def __eq__(self,o):
        return o==self.__hash__()
