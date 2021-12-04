from fastapi import APIRouter,Request,HTTPException
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from .utils import format_to_image,ImageType,db_to_json,myendpoints,myendpoints_info
import time

router=APIRouter()

"""Routes"""

@router.get("/random")
@router.get("/random/")
async def overall(request:Request,gif:bool=None,top:bool=None,many:bool=None,exclude:str=None):
    banned_files=None
    if exclude:
        try:
            banned_files=format_to_image(exclude)
        except:
            banned_files=None

    if gif is None:
        gifstr=""
    elif gif:
        gifstr="and Images.extension='.gif'"
    else:
        gifstr="and not Images.extension='.gif'"

    database=time.perf_counter()
    if not banned_files:
        fetch=await request.app.state.pool.fetch(f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT {'DISTINCT' if top else ''} Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,Images.uploaded_at, COUNT(FavImages.image) as like
    FROM LinkedTags
    JOIN Images ON Images.file=LinkedTags.image
    JOIN Tags ON Tags.id=LinkedTags.tag_id
    LEFT JOIN FavImages ON FavImages.image=Images.file
    WHERE not Images.is_banned and not Images.under_review {gifstr}
    GROUP BY Images.file,Images.extension,image_id,Images.dominant_color,Images.source,Images.uploaded_at,LinkedTags.tag_id
    ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'}
    LIMIT {'30' if many else '1'}
    ) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'ORDER BY Q.like DESC' if top else ''}""")
    else:
        # Somehow even providing a empty array increase database response time by a lot so I prefer to separate it to avoid long response time for nothing.
        fetch=await request.app.state.pool.fetch(f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,Images.uploaded_at, COUNT(FavImages.image) as like
    FROM LinkedTags
    JOIN Images ON Images.file=LinkedTags.image
    JOIN Tags ON Tags.id=LinkedTags.tag_id
    LEFT JOIN FavImages ON FavImages.image=Images.file
    WHERE not Images.is_banned and not Images.under_review {gifstr} and not Images.file = any($2::VARCHAR[])
    GROUP BY Images.file,LinkedTags.tag_id
    ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'}
    LIMIT {'30' if many else '1'}
    ) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'ORDER BY Q.like DESC' if top else ''}""",[im.filename for im in banned_files])
    jsonformating=time.perf_counter()
    images=db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not images:
        print("Criteria error for random")
        raise HTTPException(status_code=404,detail=f"Sorry there is no image matching your criteria. Please change the criteria.")
    print(f"Files :"+'\n'.join([im['file']+im['extension'] for im in images]))
    return JSONResponse(dict(code=200,images=images))

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
    if category==14 or category=="all":
        raise HTTPException(status_code=404,detail=f"Sorry, the all tag has been removed since it do not makes sense to keep it anymore, and can lead to missunderstandings. the default tag for sfw will be waifu.")
    if gif is None:
        gifstr=""
    elif gif:
        gifstr="and Images.extension='.gif'"
    else:
        gifstr="and not Images.extension='.gif'"
    database=time.perf_counter()
    if not banned_files:
        fetch=await request.app.state.pool.fetch(f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,Images.uploaded_at, COUNT(FavImages.image) as like
    FROM LinkedTags
    JOIN Images ON Images.file=LinkedTags.image
    JOIN Tags ON Tags.id=LinkedTags.tag_id
    LEFT JOIN FavImages ON FavImages.image=Images.file
    WHERE not Images.is_banned and not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw {gifstr}
    GROUP BY Images.file,LinkedTags.tag_id
    ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'}
    LIMIT {'30' if many else '1'}
    ) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'ORDER BY Q.like DESC' if top else ''}""",category)
    else:
        #somehow even providing a empty array increase database answer by a lot so i prefer to separate it for now.
        fetch=await request.app.state.pool.fetch(f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,Images.uploaded_at, COUNT(FavImages.image) as like
    FROM LinkedTags
    JOIN Images ON Images.file=LinkedTags.image
    JOIN Tags ON Tags.id=LinkedTags.tag_id
    LEFT JOIN FavImages ON FavImages.image=Images.file
    WHERE not Images.is_banned and not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw {gifstr} and not Images.file = any($2::VARCHAR[])
    GROUP BY Images.file,LinkedTags.tag_id
    ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'}
    LIMIT {'30' if many else '1'}
    ) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'ORDER BY Q.like DESC' if top else ''}""",category,[im.filename for im in banned_files])
    jsonformating=time.perf_counter()
    images=db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not images:
        print(f"This request for {category} ended in criteria error.")
        raise HTTPException(status_code=404,detail=f"Sorry there is no {itype} image matching your criteria with the tag : {category}. Please change the criteria or consider changing your tag.")
    print(f"Files :"+'\n'.join([im['file']+im['extension'] for im in images]))
    return JSONResponse(dict(code=200,images=images))

@router.get("/info")
@router.get("/info/")
async def image_info(request:Request,images:str):
    """Image infos"""
    images=format_to_image(images)
    image_infos = await request.app.state.pool.fetch("""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,Images.uploaded_at, COUNT(FavImages.image) as like
    FROM LinkedTags
    JOIN Images ON Images.file=LinkedTags.image
    JOIN Tags ON Tags.id=LinkedTags.tag_id
    LEFT JOIN FavImages ON FavImages.image=Images.file
    WHERE not Images.is_banned and not Images.under_review and Images.file = any($1::VARCHAR[])
    GROUP BY Images.file,LinkedTags.tag_id
    ) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id""",[image.filename for image in images])
    if not image_infos:
        raise HTTPException(404,detail="Sorry you did not provide any valid filename.")
    infos=db_to_json(image_infos)
    return dict(images=infos)

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
