from utils import convert_bool,myendpoints,myendpoints_info,Tags,format_to_image,wich_action,methodandimage,db_to_json
from uvicorn.middleware.proxy_headers import ProxyHeadersMiddleware
from itsdangerous import URLSafeSerializer, BadSignature
from quart import Quart,jsonify,request,current_app
from werkzeug.exceptions import HTTPException
from discord.ext import tasks
import time
import asyncpg
import functools
import asyncio
import aiohttp
import random
import quart
import json
import copy
import os

app = Quart(__name__)
app.asgi_app = ProxyHeadersMiddleware(app.asgi_app, trusted_hosts=["127.0.0.1"])

with open("json/credentials.json",'r') as f:
    dt=json.load(f)
    db_user=dt['db_user']
    db_password=dt['db_password']
    db_ip=dt['db_ip']
    db_name=dt['db_name']
    app.secret_key=dt['secret_key']

async def create_session(app):
    app.session=aiohttp.ClientSession()
    app.pool = await asyncpg.create_pool(user=db_user,password=db_password,host=db_ip,database=db_name)

app.pool=None
app.config['JSON_SORT_KEYS'] = False

"""tools"""


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

"""Token verification"""
def requires_token_authorization(view):
    """A decorator for quart views which return a 401 if token is invalid"""

    @functools.wraps(view)
    async def wrapper(*args, **kwargs):
        await is_valid_token(request.headers.get('Authorization'),request_perms=request.args.get('id'))
        return await view(*args, **kwargs)
    return wrapper

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
        if request_perms:
            perm_name="access_galleries"
            authorized=await app.pool.fetchrow('SELECT Registered_user.is_admin,Permissions.page FROM Registered_user LEFT JOIN Permissions ON Permissions.user_id=Registered_user.id WHERE Registered_user.id=$1 and Registered_user.secret=$2 and (Permissions.page=$3 or Registered_user.is_admin) ',user_id,user_secret,perm_name)
        else:
            authorized=await app.pool.fetchrow('SELECT id,is_admin from Registered_user WHERE id=$1 and secret=$2 ',user_id,user_secret)
        if authorized:
            return True
        else:
            quart.abort(403,description=f"Invalid Token, You do not have the permissions to request this route please check that the token is up to date{' and, as you requested the id url parameter that you have the permissions to do so' if request_perms else ''}.")


"""Routes"""
@app.route("/<typ>/<categorie>/")
async def principal(typ,categorie):
    gif=request.args.get('gif',type=convert_bool)
    many=request.args.get("many",type=convert_bool)
    banned_files=request.args.get("exclude",type=format_to_image)
    autho=["nsfw","sfw"]
    typ=typ.lower()
    category_str=False
    categorie=str(categorie)
    categorie=categorie.lower()
    try:
        categorie=int(categorie)
    except:
        category_str=True

    if typ=="nsfw":
        over18=True
    else:
        over18=False

    if typ in autho:        
        if gif==None:
            gifstr=""
        elif gif:
            gifstr=" and Images.extension='.gif'"
        else:
            gifstr=" and not Images.extension='.gif'"
        if banned_files:
            fetch=await app.pool.fetch(f"""SELECT Images.file,Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.dominant_color,Images.source FROM LinkedTags
                            JOIN Images ON Images.file=LinkedTags.image
                            JOIN Tags ON Tags.id=LinkedTags.tag_id
                            WHERE not Images.is_banned and not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw{gifstr} and not LinkedTags.image = any($2::VARCHAR[])
                            GROUP BY Images.file,Tags.name,Tags.is_nsfw ORDER BY RANDOM() LIMIT {'30' if many else '1'}""",categorie,[im.filename for im in banned_files])
        else:
            fetch=await app.pool.fetch(f"""SELECT Images.file,Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.dominant_color,Images.source FROM LinkedTags
                            JOIN Images ON Images.file=LinkedTags.image
                            JOIN Tags ON Tags.id=LinkedTags.tag_id
                            WHERE not Images.is_banned and not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw{gifstr}
                            GROUP BY Images.file,Tags.id,Tags.name,Tags.is_nsfw ORDER BY RANDOM() LIMIT {'30' if many else '1'}""",categorie)

        images=db_to_json(fetch)
        if not images:
            print(f"This request for {categorie} ended in criteria error.")
            quart.abort(404,description=f"Sorry there is no {typ} image matching your criteria with the tag : {categorie}. Please change the criteria or consider changing your tag.")
        print([im['url'] for im in images[0]['images']])
        return jsonify(code=200,tags=images,url='https://api.waifu.im/image/oldjson.png')

    return quart.abort(404)

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
            resp=await app.session.get(f'http://127.0.0.1:8033/userinfos/?id={rqst_user}')
        except:
            quart.abort(500)
        if resp.status!=200:
            quart.abort(resp.status)
        t=await resp.json()
        user_id=t.get('id')
        username=t.get('name')

    async with app.pool.acquire() as conn:
        if username:
            await conn.execute('INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2',user_id,username)
        querys=[]
        insert=request.args.get('insert',type=format_to_image,default=[])
        delete=request.args.get('delete',type=format_to_image,default=[])
        toggle=request.args.get('toggle',type=format_to_image)
        await wich_action(app,toggle,insert,delete,user_id,conn)
        if insert:
            querys.append(methodandimage(user_id,insert=insert))
        if delete:
            querys.append(methodandimage(user_id,delete=delete))
        
        for query in querys:
            await conn.executemany(query[0],query[1])
        images=await conn.fetch("""SELECT Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.file,Images.dominant_color,Images.source FROM FavImages
                            JOIN Images ON Images.file=FavImages.image
                            JOIN LinkedTags ON LinkedTags.image=FavImages.image
                            JOIN Tags on LinkedTags.tag_id=Tags.id
                            WHERE not Images.is_banned
                            and user_id=$1 ORDER BY Images.id DESC""",user_id)
    if not images and not insert and not delete:
        quart.abort(404,description="You have no Gallery or it is now empty.")
    tags_=db_to_json(images)
    if insert or delete:
        return jsonify({'tags':tags_,'inserted':[i.fullfilename for i in insert],'deleted':[d.fullfilename for d in delete]})
    return jsonify(tags=tags_)
    
@app.route("/info/")
async def image_info():
    images=request.args.get('images',type=format_to_image,default=[])
    if not images:
        quart.abort(404,description='You must provide a "images" url parameter.')
    image_infos = await app.pool.fetch("""SELECT Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.file,Images.dominant_color,Images.source FROM LinkedTags
                            JOIN Images ON Images.file=LinkedTags.image
                            JOIN Tags on LinkedTags.tag_id=Tags.id
                            WHERE not Images.is_banned and LinkedTags.image = any($1::VARCHAR[])""",[image.filename for image in images])
    if not image_infos:
        quart.abort(404,description="Sorry you did not provide any valid filename.")
    infos=db_to_json(image_infos)
    return jsonify(tags=infos)

"""endpoints with and without info"""
@app.route('/endpoints_info/')
async def endpoints_info():
    return jsonify(await myendpoints_info(app,over18=None))

@app.route('/endpoints/')
async def endpoints_():
    return jsonify(await myendpoints(app,over18=None))

@app.route('/favicon.ico/')
async def favicon():
    return quart.wrappers.response.FileBody("../website/static/images/favico.png")

if __name__ == "__main__":
    loop=asyncio.get_event_loop()
    loop.create_task(create_session(app))
    loop.run_until_complete(app.run_task(port=8034))
