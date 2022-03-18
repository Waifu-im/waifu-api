import os
import urllib
import asyncpg

from fastapi import APIRouter, Request, HTTPException, Header, Depends, Query
from fastapi_limiter.depends import RateLimiter
from .utils import (
    DEFAULT_REGEX,
    format_to_image,
    db_to_json,
    check_permissions,
    insert_fav_image,
    delete_fav_image,
    timesrate,
    perrate,
    blacklist_callback,
    get_user_info,
)

router = APIRouter()


@router.get(
    "/fav",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/fav/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def fav_(
        request: Request,
        user_id: int = None,
):
    """fetch a user favourite gallery"""
    info = await check_permissions(request=request, permissions=["manage_galleries"], check_identity_only=True,
                                   user_id=user_id)
    token_user_id = int(info["id"])
    images = await request.app.state.fetch(
        "SELECT Images.extension,Images.file,Images.id as image_id,Images.dominant_color,Images.source,"
        "Images.uploaded_at,Images.is_nsfw,Tags.name,Tags.id,Tags.description,Tags.is_nsfw as tag_is_nsfw, "
        "(SELECT COUNT(FavImages.image) FROM FavImages WHERE image=Images.file) as favourites,"
        "FavImages.added_at "
        "FROM FavImages "
        "JOIN Images ON Images.file=FavImages.image "
        "JOIN LinkedTags ON LinkedTags.image=FavImages.image "
        "JOIN Tags on LinkedTags.tag_id=Tags.id "
        "WHERE user_id=$1 ORDER BY added_at DESC",
        token_user_id,
    )
    if not images:
        raise HTTPException(
            status_code=404, detail="You have no Gallery or it is empty."
        )
    images_ = db_to_json(images)
    return dict(images=images_)


@router.post(
    "/fav/insert",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.post(
    "/fav/insert/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def fav_insert(
        request: Request,
        image: DEFAULT_REGEX = Query(...),
        user_id=Query(None),
):
    """Add an image to a user gallery"""
    image = format_to_image(image)
    user_name = None
    user_info = await check_permissions(request=request, permissions=["manage_galleries"], check_identity_only=True,
                                        user_id=user_id)
    target_id = user_info['id']
    if user_id:
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
    return


@router.delete(
    "/fav/delete",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.delete(
    "/fav/delete/",
    tags=["Galleries"],
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def fav_delete(
        request: Request,
        image: DEFAULT_REGEX = Query(...),
        user_id=Query(None),
):
    """Remove an image from a user gallery"""
    image = format_to_image(image)
    user_info = await check_permissions(request=request,permissions=["manage_galleries"], check_identity_only=True, user_id=user_id)
    target_id = user_id or user_info['id']
    async with request.app.state.pool.acquire() as connection:
        await delete_fav_image(target_id, image.file, connection)
    return


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
):
    """Remove or add an image to the user gallery, depending on if it is already in."""
    image = format_to_image(image)
    user_name = None
    user_info = await check_permissions(request=request,permissions=["manage_galleries"], check_identity_only=True, user_id=user_id)
    target_id = user_info['id']
    if user_id:
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
        res = await connection.fetchval("SELECT image FROM FavImages WHERE user_id = $1 and image = $2")
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
):
    info = await check_permissions(permissions=["report"])
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
