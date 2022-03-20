from fastapi import APIRouter, Request, HTTPException, Depends, Query, Header
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from fastapi_limiter.depends import RateLimiter
from .utils import (
    fetch_image,
    CustomBool,
    format_in,
    format_to_image,
    OrderByType,
    json_image_encoder,
    get_tags,
    timesrate,
    perrate,
    blacklist_callback,
    DEFAULT_REGEX,
    check_permissions,
)
import time
from typing import List, Optional
from pydantic import BaseModel

router = APIRouter()


class Image(BaseModel):
    pass
class Tag(BaseModel):
    pass


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
    response_model=Image,
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
    # This is inside a function since a Model is created on startup using this function (we are reusing code).
    results = await fetch_image(request.app.state.pool,
                                is_nsfw=is_nsfw,
                                selected_tags=selected_tags,
                                excluded_tags=excluded_tags,
                                excluded_files=excluded_files,
                                gif=gif,
                                order_by=order_by,
                                many=many,
                                full=full
                                )
    database_end = time.perf_counter()
    images = json_image_encoder(results)
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
    infos = json_image_encoder(image_infos)
    return dict(images=infos)


@router.get("/tags", response_model=Tag)
@router.get("/tags/", response_model=Tag)
@router.get("/endpoints", response_model=Tag)
@router.get("/endpoints/", response_model=Tag)
async def endpoints_(request: Request, full: bool = False):
    """endpoints with and without info"""
    data = await get_tags(request.app, full=full)
    return jsonable_encoder(data)


@router.get("/test")
@router.get("/test/")
async def test_():
    """A test route to see the difference in response time with and without a sql query"""
    return dict(message="test")
