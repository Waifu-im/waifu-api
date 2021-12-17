from fastapi import APIRouter, Request, HTTPException, Header, Depends
from fastapi.responses import JSONResponse
from fastapi.encoders import jsonable_encoder
from itsdangerous import URLSafeSerializer, BadSignature
from fastapi_limiter.depends import RateLimiter
from .utils import (
    format_to_image,
    db_to_json,
    is_valid_token,
    wich_action,
    create_query,
    timesrate,
    perrate,
    blacklist_callback,
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
    authorization: str = Header(None),
    info: dict = Depends(is_valid_token),
    insert: str = None,
    delete: str = None,
    toggle: str = None,
):
    """gallerys endpoint"""
    token_user_id = int(info["id"])
    token_user_secret = info["secret"]
    username = None
    if user_id:
        resp = await request.app.state.httpsession.get(
            f"http://127.0.0.1:8033/userinfos/?id={user_id}"
        )
        if resp.status == 404:
            raise HTTPException(
                status_code=400, detail="Please provide a valid user_id"
            )
        if resp.status != 200:
            raise HTTPException(
                status_code=500,
                detail="Sorry, something went wrong with the ipc request.",
            )
        t = await resp.json()
        token_user_id = t.get("id")
        username = t.get("name")

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

        for query in querys:
            await conn.executemany(query[0], query[1])
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
