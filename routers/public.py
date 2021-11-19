
from fastapi import APIRouter,Request,HTTPException
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from .utils import format_to_image,ImageType,db_to_json,myendpoints,myendpoints_info
import time

router=APIRouter()

"""Routes"""
@router.get("/{itype}/{category}")
@router.get("/{itype}/{category}/")
async def principal(request:Request,itype: ImageType,category : str,gif:bool=None,top:bool=None,many:bool=None,exclude:str=None):
    """Get a random image"""
    banned_files=None
    if exclude:
        try:
            banned_files=format_to_image(exclude)
        except:
            banned_files=None
    category_str=False
    category=category.lower()
    try:    
        categorie=int(categorie)
    except:
        category_str=True

    if itype==ImageType.nsfw:
        over18=True
    else:
        over18=False
       
    if gif is None:
        gifstr=""
    elif gif:
        gifstr=" and Images.extension='.gif'"
    else:
        gifstr=" and not Images.extension='.gif'"

    database=time.perf_counter()
    if not banned_files:
        fetch=await request.app.state.pool.fetch(f"""SELECT Images.file,Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.dominant_color,Images.source FROM LinkedTags
JOIN Images ON Images.file=LinkedTags.image
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'JOIN FavImages ON FavImages.image=LinkedTags.image' if top else ''}
WHERE not Images.is_banned and not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw{gifstr}
GROUP BY Images.file,Tags.id,Tags.name,Tags.is_nsfw ORDER BY {'count(Images.file) DESC' if top else 'RANDOM()' }  LIMIT {'30' if many else '1'}""",category)
    else:
        #somehow even providing a empty array increase database answer by a lot so i prefer to separate it for now.
        fetch=await request.app.state.pool.fetch(f"""SELECT Images.file,Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.dominant_color,Images.source FROM LinkedTags
JOIN Images ON Images.file=LinkedTags.image
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'JOIN FavImages ON FavImages.image=LinkedTags.image' if top else ''}
WHERE not Images.is_banned and not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw{gifstr} and not LinkedTags.image = any($2::VARCHAR[])
GROUP BY Images.file,Tags.id,Tags.name,Tags.is_nsfw ORDER BY {'count(Images.file) DESC' if top else 'RANDOM()' }  LIMIT {'30' if many else '1'}""",category,[im.filename for im in banned_files])
    jsonformating=time.perf_counter()
    tagsjson=db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not tagsjson:
        print(f"This request for {category} ended in criteria error.")
        raise HTTPException(status_code=404,detail=f"Sorry there is no {itype} image matching your criteria with the tag : {category}. Please change the criteria or consider changing your tag.")
    
    print(f"Files :"+'\n'.join([im['file']+im['extension'] for im in tagsjson[0]['images']]))
    return jsonable_encoder(dict(code=200,tags=tagsjson,url='https://api.waifu.im/image/oldjson.png'))

@router.get("/info")
@router.get("/info/")
async def image_info(request:Request,images:str):
    """Image infos"""
    images=format_to_image(images)
    image_infos = await request.app.state.pool.fetch("""SELECT Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.file,Images.dominant_color,Images.source FROM LinkedTags
                            JOIN Images ON Images.file=LinkedTags.image
                            JOIN Tags on LinkedTags.tag_id=Tags.id
                            WHERE not Images.is_banned and LinkedTags.image = any($1::VARCHAR[])""",[image.filename for image in images])
    if not image_infos:
        raise HTTPException(404,detail="Sorry you did not provide any valid filename.")
    infos=db_to_json(image_infos)
    return dict(tags=infos)

@router.get('/endpoints')
@router.get('/endpoints/')
async def endpoints_(request:Request,full:bool=False):
    """endpoints with and without info"""
    data=await myendpoints_info(request.app,over18=None) if full else await myendpoints(request.app,over18=None)
    return jsonable_encoder(data)

@router.get('/test')
@router.get('/test/')
async def test_():
    """A test route to see the difference in response time with and without a sql query"""
    return dict(message="this is a test")

