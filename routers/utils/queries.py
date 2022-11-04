import random

import random
import asyncpg
from fastapi import HTTPException
from .types import FavOrderByType
from .formatings import (
    format_tags_where,
    format_image_type,
    format_order_by,
    format_orientation,
    format_gif,
    format_in,
    format_limit,
)


async def fetch_image(
        connection,
        is_nsfw=False,
        selected_tags=None,
        excluded_tags=None,
        excluded_files=None,
        gif=None,
        order_by=None,
        orientation=None,
        many=None,
        full=False,
        gallery_mode=False,
        user_id=None,
):
    args_ = (user_id,) if user_id and gallery_mode else ()
    if excluded_tags is None:
        excluded_tags = set()
    if selected_tags is None:
        selected_tags = set()
    res = await connection.fetch(
        "SELECT DISTINCT Q.signature,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,"
        f"Q.is_nsfw,Q.width,Q.height,{'Q.liked_at,' if gallery_mode else ''}"
        "Tags.name,Tags.id,Tags.description,Tags.is_nsfw as tag_is_nsfw "
        "FROM ("
        "SELECT Images.signature,Images.extension,Images.image_id,Images.dominant_color,Images.source,"
        f"Images.uploaded_at,Images.is_nsfw,Images.width,Images.height, {'FavImages.liked_at,' if gallery_mode else ''}"
        "(SELECT COUNT(image_id) from FavImages WHERE image_id=Images.image_id) as favourites "
        "FROM Images JOIN LinkedTags ON Images.image_id=LinkedTags.image_id JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{'JOIN FavImages ON FavImages.image_id=Images.image_id AND FavImages.user_id=$1' if gallery_mode else ''} "
        "WHERE not Images.under_review and not Images.hidden "
        f"{format_image_type(is_nsfw, selected_tags)} "
        f"{f'and {format_gif(gif)}' if gif is not None else ''} "
        f"{f'and {format_orientation(orientation)}' if orientation is not None else ''} "
        f"{f'and Images.image_id not in ({format_in([im.image_id for im in excluded_files if im.image_id.isdecimal()])})' if excluded_files else ''} "
        f"{f'and {format_tags_where(selected_tags, excluded_tags)}' if selected_tags or excluded_tags else ''} "
        f"GROUP BY Images.image_id{',FavImages.liked_at' if order_by == FavOrderByType.liked_at else ''} "
        f"{f'HAVING COUNT(*)={len(selected_tags)}' if selected_tags else ''} "
        f"{format_order_by(order_by)} "
        f"{format_limit(many) if not full else ''} "
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image_id=Q.image_id JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{format_order_by(order_by, table_prefix='Q.', disable_random=True)}",
        *args_
    )
    if order_by is None:
        random.shuffle(res)
    return res


async def insert_fav_image(user_id, image, conn):
    try:
        await conn.execute("INSERT INTO FavImages(user_id,image_id) VALUES($1,$2)", user_id, image)
    except asyncpg.exceptions.ForeignKeyViolationError:
        raise HTTPException(
            status_code=400,
            detail="the image you provided do not exist.",
        )
    except asyncpg.exceptions.UniqueViolationError:
        raise HTTPException(
            status_code=400,
            detail="the image you provided is already in the user favourites, consider "
                   "using /fav/toggle/ instead.",
        )


async def delete_fav_image(user_id, image, conn):
    res = await conn.fetchval("DELETE FROM FavImages WHERE user_id = $1 AND image_id = $2 RETURNING  *",
                              user_id,
                              image,
                              )
    if not res:
        raise HTTPException(
            status_code=400,
            detail="The image you provided do not exist or is not in the user favourites.",
        )
