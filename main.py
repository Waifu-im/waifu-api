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

loop=asyncio.get_event_loop()
app = Quart(__name__)

with open("json/credentials.json",'r') as f:
    dt=json.load(f)
    db_user=dt['db_user']
    db_password=dt['db_password']
    db_ip=dt['db_ip']
    db_name=dt['db_name']
    app.secret_key=dt['secret_key']


app.pool=None
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

async def is_valid_token(token_header):
    try:
        token = token_header.split(" ")[1]
        rule = URLSafeSerializer(app.secret_key)
        info=rule.loads(token)
        user_secret=info["secret"]
        user_id=int(info['id'])
    except (TypeError,KeyError,AttributeError,IndexError,BadSignature):
        quart.abort(401,description="Invalid Token, please check that your token is up to date or that you did correctly format it in the Authorization header.")

    else:
        async with app.pool.acquire() as conn:
            async with conn.cursor() as cur:
                await cur.execute("SELECT id from User WHERE id=%s and secret=%s ",(user_id,user_secret))
                authorized=await cur.fetchone()
                if authorized:
                    return True
                else:
                    quart.abort(401,description="Invalid Token, please check that your token is up to date or that you did correctly format it in the Authorization header.")

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
        await is_valid_token(request.headers.get('Authorization'))
        return await view(*args, **kwargs)
    return wrapper

    
async def myendpoints(over18=None):
    async with app.pool.acquire() as conn:
        async with conn.cursor() as cur:
            await cur.execute("SELECT name,is_over18 FROM Tags")
            rt=await cur.fetchall()
    
    if over18 is None:
        return {"sfw":[tag[0] for tag in rt if not tag[1] and tag[0]!="example"],"nsfw":[tag[0] for tag in rt if tag[1]],'example':'https://api.hori.ovh/sfw/waifu/'}
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
        return {"sfw":[{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if not tag[2] and tag[0]!="example"],"nsfw":[{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if tag[2]],'example':'https://api.hori.ovh/sfw/waifu/'}
    elif over18:
        return [{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if tag[2]]
    else:
        return [{'name':tag[0],'id':tag[1],'description':tag[3]} for tag in rt if not tag[2]]

"""Routes"""
@app.route("/<typ>/<categorie>/")
async def principal(typ,categorie):
    gif=request.args.get('gif')
    banned_files=request.args.get("filter")
    many=request.args.get("many")
    autho=["nsfw","sfw"]
    typ=typ.lower()
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
                    await cur.execute(f"""SELECT Images.file,Images.extension FROM LinkedTags
                                    JOIN Images ON Images.file=LinkedTags.image
                                    JOIN Tags ON Tags.id=LinkedTags.tag_id
                                    WHERE not Images.is_banned and not Images.under_review and Tags.name=%s and Tags.is_over18={1 if over18 else 0}{gifstr} and LinkedTags.image not in %s{' GROUP BY LinkedTags.image' if many else ''}
                                    ORDER BY RAND() LIMIT {'40' if many else '1'}""",(categorie,banned_files))
                else:
                    await cur.execute(f"""SELECT Images.file,Images.extension FROM LinkedTags
                                    JOIN Images ON Images.file=LinkedTags.image
                                    JOIN Tags ON Tags.id=LinkedTags.tag_id
                                    WHERE not Images.is_banned and not Images.under_review and Tags.name=%s and Tags.is_over18={1 if over18 else 0}{gifstr}{' GROUP BY LinkedTags.image' if many else ''}
                                    ORDER BY RAND() LIMIT {'40' if many else '1'}""",categorie)

                fetch=list(await cur.fetchall())
                file=[]
                picture=[]
                for im in fetch:
                    file.append(im["file"]+im["extension"])
                    picture.append("https://api.hori.ovh/image/"+im["file"]+im["extension"])
                if len(picture)<1:
                    quart.abort(404,description="No ressources found.")

                data={'code':200,'is_over18':over18,'file':file if len(file)>1 else file[0],'url':picture if len(picture)>1 else picture[0]}
                return jsonify(data)

    return quart.abort(404)

@app.route('/fav/')
@requires_token_authorization
async def fav_():
    token_header = request.headers.get('Authorization')
    token = token_header.split(" ")[1]
    rule = URLSafeSerializer(app.secret_key)
    info=rule.loads(token)
    user_secret=info["secret"]
    user_id=int(info['id'])
    async with app.pool.acquire() as conn:
        async with conn.cursor(aiomysql.DictCursor) as cur:
            await cur.execute("""SELECT Images.file,Images.extension,Tags.name,Tags.is_over18 FROM FavImages
                                JOIN Images ON Images.file=FavImages.image
                                JOIN LinkedTags ON LinkedTags.image=FavImages.image
                                JOIN Tags on LinkedTags.tag_id=Tags.id
                                WHERE not Images.is_banned
                                and user_id=%s""",user_id)
            images=await cur.fetchall()
    if not images:
        quart.abort(404,description="You have no Gallery or it is empty.")
    all_=[]
    tags_nsfw={}
    tags_sfw={}
    default_tags={'ero':tags_nsfw,'waifu':tags_sfw}
    for im in images:
        filename=im['file']+im["extension"]
        if not im["is_over18"]:
            if not im["name"] in tags_sfw:
                tags_sfw.update({im["name"]:[]})
            tags_sfw[im["name"]].append(f"https://api.hori.ovh/image/{filename}")
        else:
            if not im["name"] in tags_nsfw:
                tags_nsfw.update({im["name"]:[]})
            tags_nsfw[im["name"]].append(f"https://api.hori.ovh/image/{filename}")

    files={}
    if tags_sfw:
        all_.extend(tags_sfw['waifu'])
        files.update({'sfw':tags_sfw})
    if tags_nsfw:
        all_.extend(tags_nsfw['ero'])
        files.update({'nsfw':tags_nsfw})
    if all_:
        files.update({'url':all_})
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
    return quart.wrappers.response.FileBody("/var/www/virtual_hosts/pics.hori.ovh/favicon/hori_final.ico")

if __name__ == "__main__":
    get_db.start()
    loop.run_until_complete(app.run_task(port=8034))
