from fastapi import APIRouter, Request, HTTPException, Depends, Query, Header
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from fastapi_limiter.depends import RateLimiter
from .utils import (
    CustomBool,
    format_tags_where,
    format_image_type,
    format_order_by,
    format_gif,
    format_in,
    format_to_image,
    OrderByType,
    db_to_json,
    get_tags,
    timesrate,
    perrate,
    blacklist_callback,
    DEFAULT_REGEX,
    format_limit,
    check_permissions,
)
import time
from typing import List, Optional

router = APIRouter()


@router.get(
    "/random",
    tags=["Get Random Images"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/random/",
    tags=["Get Random Images"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def random_(
        request: Request,
        authorization=Header(None),
        is_nsfw: CustomBool = False,
        selected_tags: List[DEFAULT_REGEX] = Query([]),
        excluded_tags: List[DEFAULT_REGEX] = Query([]),
        excluded_files: List[DEFAULT_REGEX] = Query([]),
        gif: bool = None,
        order_by: OrderByType = None,
        many: bool = None,
        full: bool = False,

):
    if full:
        await check_permissions(request=request, permissions=["admin"], token=authorization)
    if excluded_files:
        excluded_files = [format_to_image(f) for f in excluded_files]
    selected_tags = list(dict.fromkeys(selected_tags))
    excluded_tags = list(dict.fromkeys(excluded_tags))
    database_start = time.perf_counter()
    fetch = await request.app.state.pool.fetch(
        "SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,"
        "Q.is_nsfw,Tags.name,Tags.id,Tags.description,Tags.is_nsfw as tag_is_nsfw "
        "FROM ("
        "SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,"
        "Images.uploaded_at,Images.is_nsfw,"
        "(SELECT COUNT(image) from FavImages WHERE image=Images.file) as favourites "
        "FROM Images JOIN LinkedTags ON Images.file=LinkedTags.image JOIN Tags ON Tags.id=LinkedTags.tag_id "
        "WHERE not Images.under_review and not Images.hidden "
        f"{format_image_type(is_nsfw, selected_tags)} "
        f"{f'and {format_gif(gif)}' if gif is not None else ''} "
        f"{f'and Images.file not in ({format_in([im.file for im in excluded_files])})' if excluded_files else ''} "
        f"{f'and {format_tags_where(selected_tags, excluded_tags)}' if selected_tags or excluded_tags else ''} "
        "GROUP BY Images.file "
        f"{f'HAVING COUNT(*)={len(selected_tags)}' if selected_tags else ''} "
        f"{format_order_by(order_by)} "
        f"{format_limit(many) if not full else ''} "
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{format_order_by(order_by, table_prefix='Q.', disable_random=True)}"
    )
    database_end = time.perf_counter()
    images = db_to_json(fetch)
    print(f"Database : {database_end - database_start}")
    if not images:
        print("No image found")
        raise HTTPException(status_code=404, detail=f"No image found matching the criteria given")
    images_to_return = [im["file"] + im["extension"] for im in images]
    print(f"Files :" + "\n".join(images_to_return))
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
async def image_info(request: Request, images: List[DEFAULT_REGEX] = Query(...)):
    """Image infos"""
    images = [format_to_image(image) for image in images]
    image_infos = await request.app.state.pool.fetch(
        "SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,"
        "Q.is_nsfw,Tags.name,Tags.id,Tags.description,Tags.is_nsfw as tag_is_nsfw "
        "FROM ("
        "SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,"
        "Images.uploaded_at,Images.is_nsfw,"
        "(SELECT COUNT(image) from FavImages WHERE image=Images.file) as favourites "
        "FROM Images "
        f"WHERE Images.file in ({format_in([im.file for im in images])}) "
        "GROUP BY Images.file "
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id"
    )
    if not image_infos:
        raise HTTPException(404, detail="You did not provide any valid filename.")
    infos = db_to_json(image_infos)
    return dict(images=infos)


@router.get("/tags")
@router.get("/tags/")
@router.get("/endpoints")
@router.get("/endpoints/")
async def endpoints_(request: Request, full: bool = False):
    """endpoints with and without info"""
    data = await get_tags(request.app, full=full)
    return jsonable_encoder(data)


@router.get("/test")
@router.get("/test/")
async def test_():
    """A test route to see the difference in response time with and without a sql query"""
    return dict(message="test")
