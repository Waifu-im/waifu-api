import asyncpg
from fastapi import HTTPException

from .formatings import (
    format_tags_where,
    format_image_type,
    format_order_by,
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
        many=None,
        full=False,
):
    if excluded_tags is None:
        excluded_tags = []
    if selected_tags is None:
        selected_tags = []
    return await connection.fetch(
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
            detail="the image you provided is already in the user gallery, consider "
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
            detail="The image you provided do not exist or is not in the user gallery.",
        )
