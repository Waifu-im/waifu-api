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
    return await connection.fetch(
        "SELECT DISTINCT Q.file,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,"
        f"Q.is_nsfw,Q.width,Q.height,{'Q.liked_at,' if gallery_mode else ''}"
        "Tags.name,Tags.id,Tags.description,Tags.is_nsfw as tag_is_nsfw "
        "FROM ("
        "SELECT Images.file,Images.extension,Images.id as image_id,Images.dominant_color,Images.source,"
        f"Images.uploaded_at,Images.is_nsfw,Images.width,Images.height, {'FavImages.liked_at,' if gallery_mode else ''}"
        "(SELECT COUNT(image) from FavImages WHERE image=Images.file) as favourites "
        "FROM Images JOIN LinkedTags ON Images.file=LinkedTags.image JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{'JOIN FavImages ON FavImages.image=Images.file AND FavImages.user_id=$1' if gallery_mode else ''} "
        "WHERE not Images.under_review and not Images.hidden "
        f"{format_image_type(is_nsfw, selected_tags)} "
        f"{f'and {format_gif(gif)}' if gif is not None else ''} "
        f"{f'and {format_orientation(orientation)}' if orientation is not None else ''} "
        f"{f'and Images.file not in ({format_in([im.file for im in excluded_files])})' if excluded_files else ''} "
        f"{f'and {format_tags_where(selected_tags, excluded_tags)}' if selected_tags or excluded_tags else ''} "
        f"GROUP BY Images.file{',FavImages.liked_at' if order_by == FavOrderByType.liked_at else ''} "
        f"{f'HAVING COUNT(*)={len(selected_tags)}' if selected_tags else ''} "
        f"{format_order_by(order_by)} "
        f"{format_limit(many) if not full else ''} "
        ") AS Q "
        "JOIN LinkedTags ON LinkedTags.image=Q.file JOIN Tags ON Tags.id=LinkedTags.tag_id "
        f"{format_order_by(order_by, table_prefix='Q.', disable_random=True)}",
        *args_

    )


async def insert_fav_image(user_id, image, conn):
    try:
        await conn.execute("INSERT INTO FavImages(user_id,image) VALUES($1,$2)", user_id, image)
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
    res = await conn.fetchval("DELETE FROM FavImages WHERE user_id = $1 AND image = $2 RETURNING  *",
                              user_id,
                              image,
                              )
    if not res:
        raise HTTPException(
            status_code=400,
            detail="The image you provided do not exist or is not in the user favourites.",
        )
