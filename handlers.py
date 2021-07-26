import os
import json
import random
import glob
import aiohttp
import asyncio
import aiomysql
import xxhash
import subprocess
from aiohttp import web
"""check duples"""
def fdupes():
    path=os.getcwd()
    process=subprocess.Popen([f'fdupes -rdN {path}/image/ > {path}/fdupesreport.txt'] ,shell=True)
    process.wait()
    return

class Web:
    def __init__(self):
        self.app=None

    async def myendpoints(self,over18=None):
        async with self.app.pool.acquire() as conn:
            async with conn.cursor() as cur:
                await cur.execute("SELECT * FROM Tags")
                rt=await cur.fetchall()
        
        if over18 is None:
            return {"sfw":[tag[0] for tag in rt if not tag[1] and tag[0]!="example"],"nsfw":[tag[0] for tag in rt if tag[1]],'example':'https://api.hori.ovh:8036/sfw/waifu'}
        elif over18:
            return [tag[0] for tag in rt if tag[1]]
        else:
            return [tag[0] for tag in rt if not tag[1]]
            
    async def myendpoints_info(self,over18=None):
        async with self.app.pool.acquire() as conn:
            async with conn.cursor() as cur:
                await cur.execute("SELECT * FROM Tags")
                rt=await cur.fetchall()
    
        if over18 is None:
            return {"sfw":[{'name':tag[0],'description':tag[2]} for tag in rt if not tag[1] and tag[0]!="example"],"nsfw":[{'name':tag[0],'description':tag[2]} for tag in rt if tag[1]],'example':'https://api.hori.ovh:8036/sfw/waifu'}
        elif over18:
            return [{'name':tag[0],'description':tag[2]} for tag in rt if tag[1]]
        else:
            return [{'name':tag[0],'description':tag[2]} for tag in rt if not tag[1]]

    """principal handler """

    async def principal(self,request):
        print("Receiving a request")

        filters=request.rel_url.query
        ext=None
        banned_ext=None
        banned_files=None
        many=None
        gif=False
        autho=["nsfw","sfw"]
        typ=request.match_info['typ'].lower()
        categorie=request.match_info['categorie'].lower()

        if typ=="nsfw":
            over18=True
        else:
            over18=False


        if "filter" in list(filters.keys()):
            banned_files=[os.path.splitext(x)[0] for x in request.rel_url.query["filter"].split(",")]

        
        if "gif" in list(filters.keys()):
            if request.rel_url.query["gif"].lower()=="true":
                gif=True

        if "many" in list(filters.keys()):
            if request.rel_url.query["many"].lower()=="true":
                many=True

        if typ in autho:        
            if categorie in await self.myendpoints(over18=over18):
                async with self.app.pool.acquire() as conn:
                    async with conn.cursor(aiomysql.DictCursor) as cur:
                        if banned_files:
                            await cur.execute(f"""SELECT * FROM LinkedTags
                                            JOIN Images ON Images.file=LinkedTags.image
                                            WHERE not Images.is_banned and not Images.under_review and LinkedTags.tag_name=%s{" and Images.extension='.gif'" if gif else ''} and LinkedTags.image not in %s{' GROUP BY LinkedTags.image' if many else ''}
                                            ORDER BY RAND() LIMIT {'30' if many else '1'}""",(categorie,banned_files))
                        else:
                            await cur.execute(f"""SELECT * FROM LinkedTags
                                            JOIN Images ON Images.file=LinkedTags.image
                                            WHERE not Images.is_banned and not Images.under_review and LinkedTags.tag_name=%s{" and Images.extension='.gif'" if gif else ''}{' GROUP BY LinkedTags.image' if many else ''}
                                            ORDER BY RAND() LIMIT {'40' if many else '1'}""",categorie)

                        fetch=list(await cur.fetchall())
                        file=[]
                        picture=[]
                        for im in fetch:
                            file.append(im["file"]+im["extension"])
                            picture.append("https://api.hori.ovh:8036/image/"+im["file"]+im["extension"])
                        if len(picture)<1:
                            print(f"This request for {categorie} ended in criteria error.")
                            return web.json_response({"code":404,"error":"Sorry no image were found with the criteria you gave to the API, please retry with a different criteria."},status=404)

                        data={'code':200,'file':file if len(file)>1 else file[0],'url':picture if len(picture)>1 else picture[0]}

                        print(f"Succesfully responded the request for {categorie} with link : https://api.hori.ovh:8036/image/{picture}")

                        return web.json_response(data)

        raise aiohttp.web.HTTPNotFound()



    """handle the form from the upload page in the API website"""
    async def post_upload(self,request):
        tags=[]
        reader = aiohttp.MultipartReader.from_response(request)

        while True:
            part = await reader.next()
            if part is None:
                break
            if part.name=="tags":
                tags.append(await part.text())
            if part.name=="source":
                source=await part.text()
            if part.name=="file" and 'image' in part.headers[aiohttp.hdrs.CONTENT_TYPE]:
                extension=part.headers[aiohttp.hdrs.CONTENT_TYPE].split('/')[1]
                file={'content':await part.read(decode=False),'extension':extension}

        if not source or len(source)<4:
            source="upload page | unknow"
        if not (file and source and tags):
            return web.Response(text=f"Sorry, the server did not received all the data it needed.")

        filename = xxhash.xxh3_64_hexdigest(file["content"])+ "." + file["extension"]
        the_path=f"image/{filename}"
        try:
            with open(the_path,"xb") as f:
                f.write(file["content"])
        except FileExistsError:
            return web.Response(text=f'<html><p>Sorry this file already exists, you can see it <a href="https://api.hori.ovh:8036/image/{filename}" target="_blank">here</a></p></html>.',content_type='text/html')

        loop=asyncio.get_event_loop()
        await loop.run_in_executor(None,fdupes)

        if not os.path.exists(the_path):
            return web.Response(text=f"Sorry, the file has been removed because it was considered as a dupes.")

        else:
            async with self.app.pool.acquire() as conn:
                async with conn.cursor() as cur:
                    await cur.execute("INSERT IGNORE INTO Images (file,extension,source,under_review) VALUES(%s,%s,%s,%s)",(os.path.splitext(filename)[0],os.path.splitext(filename)[1],source,1))
                    for d in tags:
                        await cur.execute("INSERT IGNORE INTO LinkedTags (image,tag_name) VALUES(%s,%s)",(os.path.splitext(filename)[0],d))
            return web.Response(text=f'<html><p>Thanks for helping us you can now find the file <a href="https://api.hori.ovh:8036/image/{filename}" target="_blank">here</a></p></html>.',content_type='text/html')

    """an endpoints that clear dupes whenever it is called"""

    async def clean_images(self,request):
        async with self.app.pool.acquire() as conn:
            async with conn.cursor() as cur:
                doublon=[]
                await cur.execute("SELECT file, extension FROM Images WHERE not is_banned")
                rt=await cur.fetchall()
                images=[im[0]+im[1] for im in rt]
                internal=os.listdir("image/")
                for image in images:
                    if not image in internal:
                        doublon.append((os.path.splitext(image)[0],))
                tt=await cur.executemany("DELETE FROM Images WHERE file=%s",doublon)
                for it in internal:
                    if not it in images:
                        doublon.append(it)
                        os.remove(it)
        content=f"Hi {request.remote}, I cleaned the duples. Here is the files that as been removed from the database and the bot storage: {','.join(doublon)}"
        return web.Response(text=content)


    """endpoints with and without info"""
    async def endpoints_info_h(self,requests):
        return web.json_response(await self.myendpoints_info(over18=None))

    async def endpoints_(self,requests):
        return web.json_response(await self.myendpoints(over18=None))

    async def favicon(self,request):
        return web.FileResponse("/var/www/virtual_hosts/pics.hori.ovh/favicon/hori_final.ico")


    def configure(self, app):
        self.app=app
        app.add_routes([web.get('/{typ}/{categorie}/',self.principal),
                        web.get('/endpoints/',self.endpoints_),
                        web.get('/endpoints_info/',self.endpoints_info_h),
                        web.get('/favicon.ico/',self.favicon),
                        web.get('/clean/',self.clean_images),
                        web.post('/post_upload/',self.post_upload)])

        #just in case of, but nginx do the work for the static files
        app.add_routes([web.static('/image/','image/'),
                        web.static('/utils/','utils/')])
