from fastapi import APIRouter, Request, HTTPException, Depends, Query
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from fastapi_limiter.depends import RateLimiter
from .utils import (
    format_tags_where,
    format_gif,
    format_image_type,
    format_in,
    format_to_image,
    OrderByType,
    db_to_json,
    get_endpoints,
    timesrate,
    perrate,
    blacklist_callback,
    DEFAULT_REGEX,
    format_limit,
    CheckFullPermissions,
)
import time
from typing import List, Optional

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
async def random_(
        request: Request,
        selected_tags: List[DEFAULT_REGEX] = Query([]),
        excluded_tags: List[DEFAULT_REGEX] = Query([]),
        excluded_files: List[DEFAULT_REGEX] = Query([]),
        gif: bool = None,
        order_by: OrderByType = None,
        is_nsfw: bool = None,
        many: bool = None,
        full: bool = Depends(CheckFullPermissions(["admin"])),

):
    if excluded_files:
        excluded_files = format_to_image(excluded_files)
    selected_tags = list(dict.fromkeys(selected_tags))
    excluded_tags = list(dict.fromkeys(excluded_tags))
    print(selected_tags)
    database_start = time.perf_counter()
    fetch = await request.app.state.pool.fetch(
        "SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,"
        "Q.is_nsfw,Tags.name,Tags.id,Tags.description "
        "FROM ("
        "SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,"
        "Images.uploaded_at,Images.is_nsfw,"
        "(SELECT COUNT(image) from FavImages WHERE image=Images.file) as favourites "
        "FROM Images JOIN LinkedTags ON Images.file=LinkedTags.image JOIN Tags ON Tags.id=LinkedTags.tag_id "
        "WHERE not Images.under_review and not Images.hidden "
        f"{f'and {format_image_type(is_nsfw)}' if is_nsfw is not None else ''} "
        f"{f'and {format_gif(gif)}' if gif is not None else ''} "
        f"{f'and Images.file not in ({format_in([im.file for im in excluded_files])})' if excluded_files else ''} "
        f"{f'and {format_tags_where(selected_tags, excluded_tags)}' if selected_tags or excluded_tags else ''} "
        "GROUP BY Images.file "
        f"{f'HAVING COUNT(*)={len(selected_tags)}' if selected_tags else ''} "
        f"ORDER BY {'favourites DESC' if order_by == OrderByType.favourite else 'RANDOM()'} "
        f"{format_limit(many) if not full else ''} "
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{'ORDER BY Q.favourites DESC' if order_by == OrderByType.favourite else ''}"
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
async def image_info(request: Request, images: List[DEFAULT_REGEX] = Query([])):
    """Image infos"""
    images = format_to_image(images)
    image_infos = await request.app.state.pool.fetch(
        f"SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,"
        "Tags.name,Tags.id,Tags.is_nsfw,Tags.description "
        "FROM (SELECT file,extension,id as image_id, COUNT(FavImages.image) as favourites,"
        "dominant_color,source,uploaded_at "
        "FROM Images "
        "LEFT JOIN FavImages ON FavImages.image=Images.file "
        f"WHERE not Images.under_review and Images.file in ({format_in([im.file for im in images])})"
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
    data = await get_endpoints(request.app, full=full)
    return jsonable_encoder(data)


@router.get("/test")
@router.get("/test/")
async def test_():
    """A test route to see the difference in response time with and without a sql query"""
    return dict(message="test")
