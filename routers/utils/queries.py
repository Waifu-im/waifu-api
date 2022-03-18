import asyncpg
from fastapi import HTTPException

async def insert_fav_image(user_id, image, conn):
    try:
        await conn.execute("INSERT INTO FavImages(user_id,image) VALUES($1,$2)", user_id, image)
    except asyncpg.exceptions.ForeignKeyViolationError:
        raise HTTPException(
            status_code=400,
            detail="One of the images you provided do not exist.",
        )
    except asyncpg.exceptions.UniqueViolationError:
        raise HTTPException(
            status_code=400,
            detail="One of the images you provided is already in the user gallery, consider "
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
