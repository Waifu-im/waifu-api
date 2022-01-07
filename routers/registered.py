import os
import urllib
import asyncpg

from fastapi import APIRouter, Request, HTTPException, Header, Depends
from fastapi_limiter.depends import RateLimiter
from .utils import (
    format_to_image,
    db_to_json,
    CheckPermissions,
    wich_action,
    create_query,
    timesrate,
    perrate,
    blacklist_callback,
    get_user_info,
)

router = APIRouter()


@router.get(
    "/fav",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
@router.get(
    "/fav/",
    dependencies=[
        Depends(
            RateLimiter(times=timesrate, seconds=perrate, callback=blacklist_callback)
        )
    ],
)
async def fav_(
    request: Request,
    user_id: int = None,
    info: dict = Depends(CheckPermissions(["manage_galleries"], grant_no_user=True)),
    insert: str = None,
    delete: str = None,
    toggle: str = None,
):
    """gallerys endpoint"""
    token_user_id = int(info["id"])
    username = None
    if user_id:
        t = await get_user_info(request.app.state.httpsession, user_id)
        token_user_id = t.get("id")
        username = t.get("full_name")

    async with request.app.state.pool.acquire() as conn:
        if username:
            await conn.execute(
                "INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2",
                token_user_id,
                username,
            )
        querys = []
        insert = format_to_image(insert)
        delete = format_to_image(delete)
        toggle = format_to_image(toggle)
        await wich_action(toggle, insert, delete, token_user_id, conn)
        if insert:
            querys.append(create_query(token_user_id, insert=insert))
        if delete:
            querys.append(create_query(token_user_id, delete=delete))

        async with conn.transaction():
            try:
                for query in querys:
                    await conn.executemany(query[0], query[1])
            except asyncpg.exceptions.ForeignKeyViolationError:
                raise HTTPException(
                    status_code=400,
                    detail="Sorry you cannot insert a non-existing image.",
                )
            except asyncpg.exceptions.UniqueViolationError:
                raise HTTPException(
                    status_code=400,
                    detail="Sorry one of the images you provided is already in the user gallery, please consider using 'toggle' query string.",
                )
        images = await conn.fetch(
            """SELECT Images.extension,Tags.name,Tags.id,Tags.is_nsfw,Tags.description,Images.file,Images.id as image_id,Images.dominant_color,Images.source, (SELECT COUNT(FavImages.image) FROM FavImages WHERE image=Images.file) as like, Images.uploaded_at,FavImages.added_at FROM FavImages
                            JOIN Images ON Images.file=FavImages.image
                            JOIN LinkedTags ON LinkedTags.image=FavImages.image
                            JOIN Tags on LinkedTags.tag_id=Tags.id
                            WHERE user_id=$1 ORDER BY added_at DESC""",
            token_user_id,
        )
    if not images and not insert and not delete:
        raise HTTPException(
            status_code=404, detail="You have no Gallery or it is empty."
        )
    images_ = db_to_json(images)
    if insert or delete:
        return dict(
            {
                "images": images_,
                "inserted": [i.filename for i in insert],
                "deleted": [d.filename for d in delete],
            }
        )
    return dict(images=images_)


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
    info: dict = Depends(CheckPermissions(["report"])),
):
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
