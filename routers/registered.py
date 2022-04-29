import os
import urllib
from typing import Set
import asyncpg

from fastapi import APIRouter, Request, HTTPException, Depends, Query
from fastapi.responses import Response
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from fastapi_limiter.depends import RateLimiter
from starlette.status import HTTP_204_NO_CONTENT

from .utils import (
    DEFAULT_REGEX,
    fetch_image,
    format_to_image,
    json_image_encoder,
    check_user_permissions,
    get_token_info,
    insert_fav_image,
    delete_fav_image,
    timesrate,
    perrate,
    blacklist_callback,
    get_user_info,
    ImageResponseModel,
    CustomBool,
    FavOrderByType,
    ImageOrientation,

)

router = APIRouter()
auth_scheme = HTTPBearer()


@router.get(
    "/fav",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
    response_model=ImageResponseModel,
)
@router.get(
    "/fav/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
    response_model=ImageResponseModel,
)
async def fav_(
        request: Request,
        is_nsfw: CustomBool = CustomBool.null,
        selected_tags: Set[DEFAULT_REGEX] = Query(set()),
        excluded_tags: Set[DEFAULT_REGEX] = Query(set()),
        excluded_files: Set[DEFAULT_REGEX] = Query(set()),
        gif: bool = None,
        order_by: FavOrderByType = FavOrderByType.liked_at,
        orientation: ImageOrientation = None,
        user_id: int = None,
        credentials: HTTPAuthorizationCredentials = Depends(auth_scheme),
):
    """fetch a user favourite gallery"""
    info = await get_token_info(request=request, token=credentials.credentials)
    target_id = info['id']
    if user_id:
        await check_user_permissions(
            request=request,
            permissions=["view_favourites"],
            user_id=info['id'],
            target_id=user_id
        )
        target_id = user_id
    selected_tags = {st.lower() for st in selected_tags if st}
    excluded_tags = {et.lower() for et in excluded_tags if et}
    excluded_files = {format_to_image(f.lower()) for f in excluded_files if f}
    images = await fetch_image(request.app.state.pool,
                                is_nsfw=is_nsfw,
                                selected_tags=selected_tags,
                                excluded_tags=excluded_tags,
                                excluded_files=excluded_files,
                                gif=gif,
                                order_by=order_by,
                                orientation=orientation,
                                full=True,
                                gallery_mode=True,
                                user_id=target_id,
                                )
    if not images:
        raise HTTPException(
            status_code=404, detail="You have no favourites or there is no image matching the criteria given."
        )
    images_ = json_image_encoder(images)
    return dict(images=images_)


@router.post(
    "/fav/insert",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
    status_code=HTTP_204_NO_CONTENT,
)
@router.post(
    "/fav/insert/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
    status_code=HTTP_204_NO_CONTENT,
)
async def fav_insert(
        request: Request,
        image: DEFAULT_REGEX = Query(...),
        user_id=Query(None),
        credentials: HTTPAuthorizationCredentials = Depends(auth_scheme),
):
    """Add an image to a user favourites."""
    image = format_to_image(image)
    user_name = None
    info = await get_token_info(request=request, token=credentials.credentials)
    target_id = info['id']
    if user_id:
        await check_user_permissions(
            request=request,
            permissions=["manage_favourites"],
            user_id=info['id'],
            target_id=user_id,
        )
        t = await get_user_info(request.app.state.httpsession, user_id)
        target_id = t.get("id")
        user_name = t.get("full_name")
    async with request.app.state.pool.acquire() as connection:
        if user_name:
            await connection.execute(
                "INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2",
                target_id,
                user_name,
            )
        await insert_fav_image(target_id, image.file, connection)
    return Response(status_code=HTTP_204_NO_CONTENT)


@router.delete(
    "/fav/delete",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
    status_code=HTTP_204_NO_CONTENT,
)
@router.delete(
    "/fav/delete/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
    status_code=HTTP_204_NO_CONTENT,
)
async def fav_delete(
        request: Request,
        image: DEFAULT_REGEX = Query(...),
        user_id=Query(None),
        credentials: HTTPAuthorizationCredentials = Depends(auth_scheme),
):
    """Remove an image from a user favourites."""
    image = format_to_image(image)
    info = await get_token_info(request=request, token=credentials.credentials)
    target_id = info['id']
    if user_id:
        await check_user_permissions(
            request=request,
            permissions=["manage_favourites"],
            user_id=info['id'],
            target_id=user_id,
        )
    async with request.app.state.pool.acquire() as connection:
        await delete_fav_image(target_id, image.file, connection)
    return Response(status_code=HTTP_204_NO_CONTENT)


@router.post(
    "/fav/toggle",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.post(
    "/fav/toggle/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def fav_toggle(
        request: Request,
        image: DEFAULT_REGEX = Query(...),
        user_id=Query(None),
        credentials: HTTPAuthorizationCredentials = Depends(auth_scheme),
):
    """Remove or add an image to the user favourites, depending on if it is already in."""
    image = format_to_image(image)
    user_name = None
    info = await get_token_info(request=request, token=credentials.credentials)
    target_id = info['id']
    if user_id:
        await check_user_permissions(
            request=request,
            permissions=["manage_favourites"],
            user_id=info['id'],
            target_id=user_id,
        )
        t = await get_user_info(request.app.state.httpsession, user_id)
        target_id = t.get("id")
        user_name = t.get("full_name")
    async with request.app.state.pool.acquire() as connection:
        if user_name:
            await connection.execute(
                "INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2",
                target_id,
                user_name,
            )
        res = await connection.fetchval("SELECT image FROM FavImages WHERE user_id = $1 and image = $2",
                                        target_id,
                                        image.file,
                                        )
        if res:
            state = "DELETED"
            await delete_fav_image(target_id, image.file, connection)
        else:
            state = "INSERTED"
            await insert_fav_image(target_id, image.file, connection)
        return dict(code=200, state=state)


@router.get(
    "/report",
    status_code=201,
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/report/",
    status_code=201,
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def report(
        request: Request,
        image: str,
        user_id: int = None,
        description: str = None,
        credentials: HTTPAuthorizationCredentials = Depends(auth_scheme),
):
    """Report an image."""
    info = await get_token_info(request=request, token=credentials.credentials)
    await check_user_permissions(
        request=request,
        permissions=["report"],
        user_id=info['id'],
    )
    existed = False
    image_name = os.path.splitext(image)[0]
    async with request.app.state.pool.acquire() as conn:
        if user_id:
            t = await get_user_info(request.app.state.httpsession, user_id)
            await conn.execute(
                "INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2",
                user_id,
                t.get("full_name"),
            )
        else:
            user_id = info["id"]
        try:
            await conn.execute(
                "INSERT INTO Reported_images (image,author_id,description) VALUES ($"
                "1,$2,$3)",
                image_name,
                user_id,
                description if not description else urllib.parse.unquote(description),
            )
        except asyncpg.exceptions.ForeignKeyViolationError:
            raise HTTPException(
                status_code=400, detail="Sorry you cannot report a non-existing image."
            )
        except asyncpg.exceptions.UniqueViolationError:
            existed = True
            res = await conn.fetchrow(
                "SELECT * FROM Reported_images WHERE image=$1", image_name
            )
            image_name = res["image"]
            user_id = res["author_id"]
            description = res["description"]

    return dict(
        image=image_name, author_id=user_id, description=description, existed=existed
    )
