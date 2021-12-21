from fastapi import APIRouter, Request, HTTPException, Depends
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from fastapi_limiter.depends import RateLimiter
from .utils import (
    format_to_image,
    ImageType,
    db_to_json,
    myendpoints,
    myendpoints_info,
    timesrate,
    perrate,
    blacklist_callback,
    MANY_LIMIT,
)
import time

router = APIRouter()


@router.get(
    "/random",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/random/",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def overall(
    request: Request,
    gif: bool = None,
    top: bool = None,
    many: bool = None,
    exclude: str = "",
):
    banned_files = None
    exclude += "," + ",".join(await request.app.state.last_images.get())
    if exclude:
        try:
            banned_files = format_to_image(exclude)
        except:
            banned_files = None

    if gif is None:
        gifstr = ""
    elif gif:
        gifstr = "and Images.extension='.gif'"
    else:
        gifstr = "and not Images.extension='.gif'"

    database = time.perf_counter()

    fetch = await request.app.state.pool.fetch(
        f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT file,extension,id as image_id, COUNT(FavImages.image) as like,dominant_color,source,uploaded_at
FROM Images
LEFT JOIN FavImages ON FavImages.image=Images.file
WHERE not Images.under_review {gifstr} {"and Images.file not in ("+",".join(["'"+im.file+"'" for im in banned_files])+")" if banned_files else ""}
GROUP BY Images.file
ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'}
LIMIT {MANY_LIMIT if many else 1}
) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'ORDER BY Q.like DESC' if top else ''}"""
    )
    jsonformating = time.perf_counter()
    images = db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not images:
        print("Criteria error for random")
        raise HTTPException(
            status_code=404,
            detail=f"Sorry there is no image matching your criteria. Please change the criteria.",
        )
    images_to_return = [im["file"] + im["extension"] for im in images]
    print(f"Files :" + "\n".join(images_to_return))
    await request.app.state.last_images.put(images_to_return)
    return JSONResponse(dict(code=200, images=images))


@router.get(
    "/{itype}/{category}",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/{itype}/{category}/",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def principal(
    request: Request,
    itype: ImageType,
    category: str,
    gif: bool = None,
    top: bool = None,
    many: bool = None,
    exclude: str = "",
):
    """Get a random image"""
    banned_files = None
    exclude += "," + ",".join(await request.app.state.last_images.get())
    if exclude:
        try:
            banned_files = format_to_image(exclude)
        except:
            banned_files = None
    category_str = False
    category = category.lower()
    try:
        category = int(categorie)
    except:
        category_str = True
    if itype == ImageType.nsfw:
        over18 = True
    else:
        over18 = False
    if category == 14 or category == "all":
        raise HTTPException(
            status_code=404,
            detail=f"Sorry, the all tag has been removed since it do not makes sense to keep it anymore, and can lead to missunderstandings. the default tag for sfw will be waifu.",
        )
    if gif is None:
        gifstr = ""
    elif gif:
        gifstr = "and Images.extension='.gif'"
    else:
        gifstr = "and not Images.extension='.gif'"
    database = time.perf_counter()
    fetch = await request.app.state.pool.fetch(
        f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,Images.uploaded_at, COUNT(FavImages.image) as like
FROM LinkedTags
JOIN Images ON Images.file=LinkedTags.image
JOIN Tags ON Tags.id=LinkedTags.tag_id
LEFT JOIN FavImages ON FavImages.image=Images.file
WHERE not Images.under_review and {'Tags.name=$1' if category_str else 'Tags.id=$1'} and {'' if over18 else 'not '}Tags.is_nsfw {gifstr} {"and Images.file not in ("+",".join(["'"+im.file+"'" for im in banned_files])+")" if banned_files else ""}
GROUP BY Images.file,LinkedTags.tag_id
ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'}
LIMIT {MANY_LIMIT if many else 1}
) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id
{'ORDER BY Q.like DESC' if top else ''}""",
        category,
    )
    jsonformating = time.perf_counter()
    images = db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not images:
        print(f"This request for {category} ended in criteria error.")
        raise HTTPException(
            status_code=404,
            detail=f"Sorry there is no {itype} image matching your criteria with the tag : {category}. Please change the criteria or consider changing your tag.",
        )
    images_to_return = [im["file"] + im["extension"] for im in images]
    print(f"Files :" + "\n".join(images_to_return))
    await request.app.state.last_images.put(images_to_return)
    return JSONResponse(dict(code=200, images=images))


@router.get(
    "/info",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/info/",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def image_info(request: Request, images: str):
    """Image infos"""
    images = format_to_image(images)
    image_infos = await request.app.state.pool.fetch(
        f"""
SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,Tags.id,Tags.is_nsfw,Tags.description
FROM (SELECT file,extension,id as image_id, COUNT(FavImages.image) as like,dominant_color,source,uploaded_at
    FROM Images
    LEFT JOIN FavImages ON FavImages.image=Images.file
    WHERE not Images.under_review and Images.file in ({",".join(["'"+im.filename+"'" for im in images])})
    GROUP BY Images.file
    ) AS Q
JOIN LinkedTags ON LinkedTags.image=Q.file
JOIN Tags ON Tags.id=LinkedTags.tag_id"""
    )
    if not image_infos:
        raise HTTPException(404, detail="Sorry you did not provide any valid filename.")
    infos = db_to_json(image_infos)
    return dict(images=infos)


@router.get("/endpoints")
@router.get("/endpoints/")
async def endpoints_(request: Request, full: bool = False):
    """endpoints with and without info"""
    data = (
        await myendpoints_info(request.app, over18=None)
        if full
        else await myendpoints(request.app, over18=None)
    )
    return jsonable_encoder(data)


@router.get("/test")
@router.get("/test/")
async def test_():
    """A test route to see the difference in response time with and without a sql query"""
    return dict(message="this is a test")
