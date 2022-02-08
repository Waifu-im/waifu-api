from fastapi import APIRouter, Request, HTTPException, Depends
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from fastapi_limiter.depends import RateLimiter
from .utils import (
    FORMAT_IMAGE_LIMIT,
    format_tag,
    format_gif,
    format_image_type,
    format_image_list,
    format_to_image,
    ImageType,
    db_to_json,
    myendpoints,
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
    banned_files = []
    if exclude:
        try:
            banned_files = format_to_image(exclude)
        except:
            raise HTTPException(
                400,
                detail=f"The maximum length for the 'exclude' query string is {FORMAT_IMAGE_LIMIT}.",
            )
    if not (gif or top):
        banned_files += format_to_image(",".join(await request.app.state.last_images.get()))
    database = time.perf_counter()
    fetch = await request.app.state.pool.fetch(
        "SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,"
        "Tags.id,Tags.is_nsfw,Tags.description "
        "FROM ("
        "SELECT file,extension,id as image_id, COUNT(FavImages.image) as like,dominant_color,source,uploaded_at "
        "FROM Images LEFT JOIN FavImages ON FavImages.image=Images.file "
        f"WHERE not Images.under_review and not Images.hidden {format_gif(gif)} "
        f"{f'and Images.file not in ({format_image_list(banned_files)})' if banned_files else ''} "
        "GROUP BY Images.file "
        f"ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'} "
        f"LIMIT {MANY_LIMIT if many else 1}"
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{'ORDER BY Q.like DESC' if top else ''}"
    )
    jsonformating = time.perf_counter()
    images = db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not images:
        print("No image found")
        raise HTTPException(
            status_code=404,
            detail=f"No image found matching the criteria given",
        )
    images_to_return = [im["file"] + im["extension"] for im in images]
    print(f"Files :" + "\n".join(images_to_return))
    if not (gif or top):
        await request.app.state.last_images.put(images_to_return)
    return JSONResponse(dict(code=200, images=images))


@router.get(
    "/{image_type}/{tag}",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/{image_type}/{tag}/",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def principal(
    request: Request,
    image_type: ImageType,
    tag: str,
    gif: bool = None,
    top: bool = None,
    many: bool = None,
    exclude: str = "",
):
    """Get a random image"""
    banned_files = []
    if exclude:
        try:
            banned_files = format_to_image(exclude)
        except:
            raise HTTPException(
                400,
                detail=f"The maximum length for the 'exclude' query string is {FORMAT_IMAGE_LIMIT}.",
            )
    if not (gif or top):
        banned_files += format_to_image(",".join(await request.app.state.last_images.get()))
    tag = tag.lower()
    database = time.perf_counter()
    fetch = await request.app.state.pool.fetch(
        "SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,"
        "Tags.id,Tags.is_nsfw,Tags.description "
        "FROM ("
        "SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,"
        "Images.uploaded_at, COUNT(FavImages.image) as like "
        "FROM LinkedTags JOIN Images ON Images.file=LinkedTags.image JOIN Tags ON Tags.id=LinkedTags.tag_id "
        "LEFT JOIN FavImages ON FavImages.image=Images.file "
        f"WHERE not Images.under_review and not Images.hidden and {format_tag(tag)} "
        f"and {format_image_type(image_type)} {format_gif(gif)} "
        f"{f'and Images.file not in ({format_image_list(banned_files)})' if banned_files else ''} "
        "GROUP BY Images.file,LinkedTags.tag_id "
        f"ORDER BY {'COUNT(FavImages.image) DESC' if top else 'RANDOM()'} "
        f"LIMIT {MANY_LIMIT if many else 1}"
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{'ORDER BY Q.like DESC' if top else ''}",
        tag,
    )
    jsonformating = time.perf_counter()
    images = db_to_json(fetch)
    print(f"Database : {jsonformating-database}")
    if not images:
        print(f"No image found.")
        raise HTTPException(
            status_code=404,
            detail=f"No {image_type} image were found for the tag '{tag}' and the criteria given.",
        )
    images_to_return = [im["file"] + im["extension"] for im in images]
    print(f"Files :" + "\n".join(images_to_return))
    if not (gif or top):
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
    try:
        images = format_to_image(images)
    except:
        raise HTTPException(
            400,
            detail=f"The maximum length for the 'images' query string is {FORMAT_IMAGE_LIMIT}.",
        )
    image_infos = await request.app.state.pool.fetch(
        f"SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.like,Q.dominant_color,Q.source,Q.uploaded_at,Tags.name,"
        "Tags.id,Tags.is_nsfw,Tags.description "
        "FROM (SELECT file,extension,id as image_id, COUNT(FavImages.image) as like,dominant_color,source,uploaded_at "
        "FROM Images "
        "LEFT JOIN FavImages ON FavImages.image=Images.file "
        f"WHERE not Images.under_review and Images.file in ({format_image_list(images)})"
        "GROUP BY Images.file) AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id"
    )
    if not image_infos:
        raise HTTPException(404, detail="You did not provide any valid filename.")
    infos = db_to_json(image_infos)
    return dict(images=infos)


@router.get("/endpoints")
@router.get("/endpoints/")
async def endpoints_(request: Request, full: bool = False):
    """endpoints with and without info"""
    data = await myendpoints(request.app, over18=None, full=full)
    return jsonable_encoder(data)


@router.get("/test")
@router.get("/test/")
async def test_():
    """A test route to see the difference in response time with and without a sql query"""
    return dict(message="test")
