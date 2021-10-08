from werkzeug.datastructures import MultiDict
import aiohttp
import json
import os


"""Endpoints infos"""
async def myendpoints(app,over18=None):
    async with app.pool.acquire() as conn:
        async with conn.cursor() as cur:
            await cur.execute("SELECT name,is_over18 FROM Tags")
            rt=await cur.fetchall()
    
    if over18 is None:
        return {"sfw":[tag[0] for tag in rt if not tag[1] and tag[0]!="example"],"nsfw":[tag[0] for tag in rt if tag[1]],'example':'https://api.waifu.im/sfw/waifu/'}
    elif over18:
        return [tag[0] for tag in rt if tag[1]]
    else:
        return [tag[0] for tag in rt if not tag[1]]


async def myendpoints_info(app,over18=None):
    async with app.pool.acquire() as conn:
        async with conn.cursor() as cur:
            await cur.execute("SELECT name,id,is_over18,description FROM Tags")
            rt=await cur.fetchall()
    if over18 is None:
        return {"sfw":[{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if not tag[2] and tag[0]!="example"],
                "nsfw":[{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if tag[2]],'example':'https://api.waifu.im/sfw/waifu/'}
    elif over18:
        return [{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if tag[2]]
    else:
        return [{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if not tag[2]]

async def create_session(app):
    app.session=aiohttp.ClientSession()



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



"""Determine if an image is already or not in the User gallery for the toggle url param"""
async def wich_action(app,image,insert,delete,user_id,cursor):
    if not image:
        return
    async with app.pool.acquire() as conn:
        async with conn.cursor() as cur:
            for im in image:
                await cursor.execute("SELECT image FROM FavImages WHERE user_id=%s and image=%s",(user_id,im.filename))
                rt=await cursor.fetchone()
                if rt:
                    delete.append(im)
                else:
                    insert.append(im)

"""Utils to format into apropriate sql qury"""
def methodandimage(user_id,insert=None,delete=None):
    
    if insert:
        args=[(user_id,im.filename) for im in insert]
        return "INSERT IGNORE INTO FavImages(user_id,image) VALUES(%s,%s)",args
    elif delete:
        args=[(user_id,im.filename) for im in delete]
        return "DELETE FROM FavImages WHERE user_id=%s and image=%s",args

def db_to_json(images):
    tagmapping=[]
    for im in images:
        tagmapping.append((Tags(im.pop('id'),im.pop('name'),im.pop('is_over18'),im.pop('description')),im))
    tagmapping=MultiDict(tagmapping)
    default_tags=['ero','all']
    tags_=[]
    for tag,image in tagmapping.items():
        tag_images=tagmapping.getlist(tag.tag_id)
        tag_images=[dict(t,**{'url':'https://api.waifu.im/image/'+t['file']+t['extension']}) for t in tag_images]
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
    def __init__(self,tag_id,name,is_over18,description):
        self.tag_id=tag_id
        self.name=name
        self.is_over18=bool(is_over18)
        self.description=description
    def __hash__(self):
        return self.tag_id
    def __eq__(self,o):
        return o==self.__hash__()
