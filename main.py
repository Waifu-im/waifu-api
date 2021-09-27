import os
import json
import random
import asyncio
import aiomysql
import itsdangerous
from itsdangerous import URLSafeSerializer, BadSignature
import quart
import aiomysql
import functools
from quart import Quart,jsonify,request,current_app
from werkzeug.exceptions import HTTPException
from discord.ext import tasks
import copy
import aiohttp
from uvicorn.middleware.proxy_headers import ProxyHeadersMiddleware
loop=asyncio.get_event_loop()
app = Quart(__name__)

app.asgi_app = ProxyHeadersMiddleware(app.asgi_app, trusted_hosts=["127.0.0.1"])

with open("json/credentials.json",'r') as f:
    dt=json.load(f)
    db_user=dt['db_user']
    db_password=dt['db_password']
    db_ip=dt['db_ip']
    db_name=dt['db_name']
    app.secret_key=dt['secret_key']


app.pool=None
app.session=aiohttp.ClientSession()
app.config['JSON_SORT_KEYS'] = False


"""tools"""

@tasks.loop(minutes=30)
async def get_db():
    loop=asyncio.get_event_loop()
    if app.pool:
        await app.pool.clear()
    else:
        app.pool = await aiomysql.create_pool(user=db_user,password=db_password,host=db_ip,db=db_name,connect_timeout=10,loop=loop,autocommit=True)


def convert_bool(string):
    string=string.lower()
    try:
        string=json.loads(string)
    except:
        return None
    return string

def format_to_list(images):
    if not images:
        return []
    images=images.lower()
    try:
        return [os.path.splitext(x)[0] for x in images.split(",")]
    except IndexError:
        return []

async def wich_action(image,insert,delete,user_id,cursor):
    async with app.pool.acquire() as conn:
        async with conn.cursor() as cur:
            for im in image:
                await cursor.execute("SELECT image FROM FavImages WHERE user_id=%s and image=%s",(user_id,im))
                rt=await cursor.fetchone()
                if rt:
                    delete.append(im)
                else:
                    insert.append(im)
def methodandimage(user_id,insert=None,delete=None):
    
    if insert:
        args=[(user_id,im) for im in insert]
        return "INSERT IGNORE INTO FavImages(user_id,image) VALUES(%s,%s)",args
    elif delete:
        args=[(user_id,im) for im in delete]
        return "DELETE FROM FavImages WHERE user_id=%s and image=%s",args

async def is_valid_token(token_header,request_perms=None):
    if not token_header:
        return quart.abort(401,description="No Token, please check that you provided a Token and that your correctly requested the route with a trailing slash (the Authorization header being not kept if you didn't).")

    try:
        token = token_header.split(" ")[1]
        rule = URLSafeSerializer(app.secret_key)
        info=rule.loads(token)
        user_secret=info.get("secret")
        user_id=int(info.get('id'))
    except (TypeError,KeyError,AttributeError,IndexError,BadSignature):
        quart.abort(403,description=f"Invalid Token, please check that you did correctly format it in the Authorization header and that the token is up to date.")

    else:
        async with app.pool.acquire() as conn:
            async with conn.cursor() as cur:
                if request_perms:
                    perm_name="access_galleries"
                    await cur.execute("SELECT User.is_admin,Permissions.page FROM User LEFT JOIN Permissions ON Permissions.user_id=User.id WHERE User.id=%s and User.secret=%s and (Permissions.page=%s or User.is_admin) ",(user_id,user_secret,perm_name))
                else:
                    await cur.execute("SELECT id,is_admin from User WHERE id=%s and secret=%s ",(user_id,user_secret))
                authorized=await cur.fetchone()
                if authorized:
                    return True
                else:
                    quart.abort(403,description=f"Invalid Token, You do not have the permissions to request this route please check that the token is up to date{' and, as you requested the id url parameter that you have the permissions to do so' if request_perms else ''}.")

"""error handlers"""



@app.errorhandler(HTTPException)
def handle_exception(e):
    response = e.get_response()
    response.data = json.dumps({
        "code": e.code,
        "name": e.name,
        "error": e.description,
    })
    response.content_type = "application/json"
    return response

def requires_token_authorization(view):
    """A decorator for quart views which return a 401 if token is invalid"""

    @functools.wraps(view)
    async def wrapper(*args, **kwargs):
        await is_valid_token(request.headers.get('Authorization'),request_perms=request.args.get('id'))
        return await view(*args, **kwargs)
    return wrapper

    
async def myendpoints(over18=None):
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


async def myendpoints_info(over18=None):
    async with app.pool.acquire() as conn:
        async with conn.cursor() as cur:
            await cur.execute("SELECT name,id,is_over18,description FROM Tags")
            rt=await cur.fetchall()

    if over18 is None:
        return {"sfw":[{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if not tag[2] and tag[0]!="example"],"nsfw":[{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if tag[2]],'example':'https://api.waifu.im/sfw/waifu/'}
    elif over18:
        return [{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if tag[2]]
    else:
        return [{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if not tag[2]]

"""Routes"""
@app.route("/<typ>/<categorie>/")
async def principal(typ,categorie):
    gif=request.args.get('gif')
    banned_files=request.args.get("exclude")
    many=request.args.get("many")
    autho=["nsfw","sfw"]
    typ=typ.lower()
    categorie=str(categorie)
    categorie=categorie.lower()
    if gif:
        gif=convert_bool(gif)
    if many:
        many=convert_bool(many) 
    if banned_files:
        banned_files=[os.path.splitext(x)[0] for x in banned_files.split(",")]
    if typ=="nsfw":
        over18=True
    else:
        over18=False


    if typ in autho:        
        async with app.pool.acquire() as conn:
            async with conn.cursor(aiomysql.DictCursor) as cur:
                if gif==None:
                    gifstr=""
                elif gif:
                    gifstr=" and Images.extension='.gif'"
                else:
                    gifstr=" and not Images.extension='.gif'"
                if banned_files:
                    await cur.execute(f"""SELECT Images.file,Images.extension,Images.dominant_color,Tags.id,Tags.name FROM LinkedTags
                                    JOIN Images ON Images.file=LinkedTags.image
                                    JOIN Tags ON Tags.id=LinkedTags.tag_id
                                    WHERE not Images.is_banned and not Images.under_review and (Tags.id=%s or Tags.name=%s) and Tags.is_over18={1 if over18 else 0}{gifstr} and LinkedTags.image not in %s{' GROUP BY LinkedTags.image' if many else ''}
                                    ORDER BY RAND() LIMIT {'30' if many else '1'}""",(categorie,categorie,banned_files))
                else:
                    await cur.execute(f"""SELECT Images.file,Images.extension,Images.dominant_color,Tags.id,Tags.name FROM LinkedTags
                                    JOIN Images ON Images.file=LinkedTags.image
                                    JOIN Tags ON Tags.id=LinkedTags.tag_id
                                    WHERE not Images.is_banned and not Images.under_review and (Tags.id=%s or Tags.name=%s) and Tags.is_over18={1 if over18 else 0}{gifstr}{' GROUP BY LinkedTags.image' if many else ''}
                                    ORDER BY RAND() LIMIT {'30' if many else '1'}""",(categorie,categorie))

                fetch=list(await cur.fetchall())
                file=[]
                picture=[]
                dominant_color=[]
                for im in fetch:
                    file.append(im["file"]+im["extension"])
                    picture.append("https://api.waifu.im/image/"+im["file"]+im["extension"])
                    dominant_color.append(im['dominant_color'])
                if len(picture)<1:
                    print(f"This request for {categorie} ended in criteria error.")
                    quart.abort(404,description=f"Sorry there is no {typ} image matching your criteria with the tag : {categorie}. Please change the criteria or consider changing your tag.")
                tag_id=fetch[0]['id']
                tag_name=fetch[0]['name']
                print(f'File: {file if len(file)>1 else file[0]}')
                
                data={'code':200,'is_over18':over18,'tag_id':tag_id,'tag_name':tag_name,'file':file if len(file)>1 else file[0],'dominant_color':dominant_color if len(dominant_color)>1 else dominant_color[0],'url':picture if len(picture)>1 else picture[0]}
                return jsonify(data)

    return quart.abort(404,f"Sorry there isn't any type named : {typ}. Please retry with either {' or '.join(autho)}.")

@app.route('/fav/')
@requires_token_authorization
async def fav_():
    token_header = request.headers.get('Authorization')
    token = token_header.split(" ")[1]
    rule = URLSafeSerializer(app.secret_key)
    info=rule.loads(token)
    user_id=int(info['id'])
    user_secret=info["secret"]
    username=None
    rqst_user=request.args.get('id')
    if rqst_user:
        try:
            resp=await app.session.get(f'127.0.0.1:8033/userinfos/?id={rqst_user}')
        except:
            quart.abort(500)
        if resp.status!=200:
            quart.abort(500)
        t=await resp.json()
        user_id=t.get('id')
        username=t.get('name')

    async with app.pool.acquire() as conn:
        async with conn.cursor(aiomysql.DictCursor) as cur:
            if username:
                await cur.execute("INSERT IGNORE INTO User(id,name) VALUES(%s,%s) ON DUPLICATE KEY UPDATE name=%s",(user_id,username,username))
            querys=[]
            insert=format_to_list(request.args.get('insert')) 
            delete=format_to_list(request.args.get('delete')) 
            toggle=format_to_list(request.args.get('toggle'))
            if toggle:
                await wich_action(toggle,insert,delete,user_id,cur)
            if insert:
                querys.append(methodandimage(user_id,insert=insert))
            if delete:
                querys.append(methodandimage(user_id,delete=delete))
            
            for query in querys:
                await cur.executemany(query[0],query[1])
            await cur.execute("""SELECT Images.extension,Tags.name,Tags.id,Tags.is_over18,Tags.description,Images.file,Images.dominant_color FROM FavImages
                                JOIN Images ON Images.file=FavImages.image
                                JOIN LinkedTags ON LinkedTags.image=FavImages.image
                                JOIN Tags on LinkedTags.tag_id=Tags.id
                                WHERE not Images.is_banned
                                and user_id=%s""",user_id)
            images=await cur.fetchall()
    if not images:
        quart.abort(404,description="You have no Gallery or it is now empty.")
    all_u=[]
    all_f=[]
    all_d=[]
    tags_nsfw={}
    tags_sfw={}
    default_tags={'ero':tags_nsfw,'all':tags_sfw}
    for im in images:
        filename=im['file']+im["extension"]
        url=f"https://api.waifu.im/image/{filename}"
        domincolor=im['dominant_color']
        if not im["is_over18"]:
            if not im["name"] in tags_sfw:
                newtag=copy.deepcopy(im)
                del newtag['extension']
                newtag['file']=[]
                newtag['dominant_color']=[]
                newtag['is_over18']=True if newtag['is_over18'] else False
                newtag.update({'url':[]})
                tags_sfw[im["name"]]=newtag
            tags_sfw[im["name"]]['dominant_color'].append(domincolor)
            tags_sfw[im["name"]]['url'].append(url)
            tags_sfw[im["name"]]['file'].append(filename)
        else:
            if not im["name"] in tags_nsfw:
                newtag=copy.deepcopy(im)
                del newtag['extension']
                newtag['file']=[]
                newtag['dominant_color']=[]
                newtag['is_over18']=True if newtag['is_over18'] else False
                newtag.update({'url':[]})
                tags_nsfw[im["name"]]=newtag
            tags_nsfw[im["name"]]['dominant_color'].append(domincolor)
            tags_nsfw[im["name"]]['url'].append(url)
            tags_nsfw[im["name"]]['file'].append(filename)

    files={}
    if tags_sfw:
        all_f.extend(tags_sfw['all']['file'])
        all_d.extend(tags_sfw['all']['dominant_color'])
        all_u.extend(tags_sfw['all']['url'])
        files.update({'sfw':tags_sfw})
    if tags_nsfw:
        all_f.extend(tags_nsfw['ero']['file'])
        all_d.extend(tags_nsfw['ero']['dominant_color'])
        all_u.extend(tags_nsfw['ero']['url'])
        files.update({'nsfw':tags_nsfw})
    if all_f and all_u:
        files.update({'file':all_f,'dominant_color':all_d,'url':all_u})
    return jsonify(files)

"""endpoints with and without info"""
@app.route('/endpoints_info/')
async def endpoints_info():
    return jsonify(await myendpoints_info(over18=None))

@app.route('/endpoints/')
async def endpoints_():
    return jsonify(await myendpoints(over18=None))

@app.route('/favicon.ico/')
async def favicon():
    return quart.wrappers.response.FileBody("../website/static/images/favico.png")

if __name__ == "__main__":
    get_db.start()
    loop.run_until_complete(app.run_task(port=8034))
